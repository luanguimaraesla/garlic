package httpclient

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
)

// readableInMemory returns the full contents of a reader backed by an in-memory
// buffer, reporting ok=false for any other reader. Such bodies can be replayed
// on retry, unlike an arbitrary one-shot stream.
func readableInMemory(r io.Reader) (data []byte, ok bool) {
	switch r.(type) {
	case *bytes.Reader, *bytes.Buffer, *strings.Reader:
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, false
		}
		return data, true
	default:
		return nil, false
	}
}

// bodyProvider yields the request body for one send attempt. A replayable
// provider returns a fresh reader on each call so the body can be replayed on
// retry; a non-replayable provider (a one-shot stream) returns its reader once
// and reports replayable=false, so the retry loop will not resend a drained body.
type bodyProvider interface {
	// body returns a reader for the body, the content type to set when the
	// caller has not set one (empty to leave it unset), the content length
	// (negative for unknown/chunked), and whether the body can be replayed.
	body() (rc io.ReadCloser, contentType string, contentLength int64, replayable bool, err error)
}

// memoryBody is a fully buffered, replayable body (JSON, raw bytes, string, or
// form-encoded). Each call to body returns a fresh reader over the same bytes.
type memoryBody struct {
	data        []byte
	contentType string
}

func (b *memoryBody) body() (io.ReadCloser, string, int64, bool, error) {
	return io.NopCloser(bytes.NewReader(b.data)), b.contentType, int64(len(b.data)), true, nil
}

// streamBody is a single-pass streamed body. It is never replayed, so a large
// upload is not buffered in memory.
type streamBody struct {
	reader      io.Reader
	size        int64 // negative when unknown.
	contentType string
}

func (b *streamBody) body() (io.ReadCloser, string, int64, bool, error) {
	if rc, ok := b.reader.(io.ReadCloser); ok {
		return rc, b.contentType, b.size, false, nil
	}
	return io.NopCloser(b.reader), b.contentType, b.size, false, nil
}

// multipartField is a single part of a multipart body: either a plain text
// field (reader nil) or a file part (reader set).
type multipartField struct {
	field    string
	filename string
	value    string
	reader   io.Reader
}

// multipartBody streams a multipart/form-data body through an io.Pipe so files
// are not buffered. It is not replayable.
type multipartBody struct {
	fields []multipartField
}

func (b *multipartBody) body() (io.ReadCloser, string, int64, bool, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	go func() {
		_ = pw.CloseWithError(writeMultipart(mw, b.fields))
	}()

	return pr, contentType, -1, false, nil
}

func writeMultipart(mw *multipart.Writer, fields []multipartField) error {
	for i := range fields {
		f := &fields[i]
		if f.reader != nil {
			w, err := mw.CreateFormFile(f.field, f.filename)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, f.reader); err != nil {
				return err
			}
			continue
		}
		if err := mw.WriteField(f.field, f.value); err != nil {
			return err
		}
	}
	return mw.Close()
}

func formProvider(values map[string]string) *memoryBody {
	form := url.Values{}
	for k, v := range values {
		form.Set(k, v)
	}
	return &memoryBody{
		data:        []byte(form.Encode()),
		contentType: "application/x-www-form-urlencoded",
	}
}
