//go:build unit

package httpclient

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestBody_jsonSetsContentTypeAndLength(t *testing.T) {
	var (
		contentType string
		length      int64
		body        []byte
	)
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		contentType = req.Header.Get("Content-Type")
		length = req.ContentLength
		body, _ = io.ReadAll(req.Body)
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	if _, err := c.R(context.Background()).SetBody(map[string]string{"a": "b"}).Post("/x"); err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q", contentType)
	}
	if length != int64(len(body)) {
		t.Errorf("Content-Length = %d, body len = %d", length, len(body))
	}
	if string(body) != `{"a":"b"}` {
		t.Errorf("body = %s", body)
	}
}

func TestBody_streamSetsExplicitContentLength(t *testing.T) {
	var length int64
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		length = req.ContentLength
		_, _ = io.Copy(io.Discard, req.Body)
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	if _, err := c.R(context.Background()).SetBodyStream(strings.NewReader("0123456789"), 10).Put("/x"); err != nil {
		t.Fatal(err)
	}
	if length != 10 {
		t.Errorf("ContentLength = %d, want 10", length)
	}
}

func TestBody_formData(t *testing.T) {
	var (
		contentType string
		body        []byte
	)
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		contentType = req.Header.Get("Content-Type")
		body, _ = io.ReadAll(req.Body)
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	if _, err := c.R(context.Background()).SetFormData(map[string]string{"q": "go lang"}).Post("/x"); err != nil {
		t.Fatal(err)
	}
	if contentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", contentType)
	}
	if string(body) != "q=go+lang" {
		t.Errorf("body = %s", body)
	}
}

func TestBody_multipartStreamsFields(t *testing.T) {
	var (
		contentType string
		body        []byte
	)
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		contentType = req.Header.Get("Content-Type")
		body, _ = io.ReadAll(req.Body)
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	_, err := c.R(context.Background()).
		SetMultipartField("field", "value").
		SetFileReader("file", "f.txt", strings.NewReader("filedata")).
		Post("/upload")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	_, params, _ := mime.ParseMediaType(contentType)
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if form.Value["field"][0] != "value" {
		t.Errorf("text field = %v", form.Value["field"])
	}
	fh := form.File["file"][0]
	f, _ := fh.Open()
	fb, _ := io.ReadAll(f)
	if string(fb) != "filedata" {
		t.Errorf("file content = %s", fb)
	}
}

func TestBody_replayedByteIdenticalOnRetry(t *testing.T) {
	crt := &countingRoundTripper{
		respond: func(attempt int) (*http.Response, error) {
			if attempt == 0 {
				return textResponse(http.StatusServiceUnavailable, "", nil), nil
			}
			return textResponse(http.StatusOK, "{}", nil), nil
		},
	}
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(2)})

	if _, err := c.R(context.Background()).SetBodyBytes([]byte("payload")).Put("/x"); err != nil {
		t.Fatal(err)
	}
	if crt.count() != 2 {
		t.Fatalf("attempts = %d, want 2", crt.count())
	}
	for i, b := range crt.bodies {
		if string(b) != "payload" {
			t.Errorf("attempt %d body = %q, want payload (body not replayed intact)", i, b)
		}
	}
}

func TestBody_nonReplayableStreamNotRetried(t *testing.T) {
	crt := &countingRoundTripper{
		respond: func(int) (*http.Response, error) {
			return textResponse(http.StatusServiceUnavailable, "", nil), nil
		},
	}
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(3)})

	resp, err := c.R(context.Background()).SetBodyStream(strings.NewReader("data"), 4).Put("/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if crt.count() != 1 {
		t.Errorf("a non-replayable stream must not be retried, attempts = %d", crt.count())
	}
}
