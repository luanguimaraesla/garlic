package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
)

// Response wraps an *http.Response. By default the send pipeline fully reads and
// closes the body, so Bytes/Decode work and the connection returns to the pool;
// under SetDoNotParseResponse the body is left open and Body streams it (the
// caller must Close).
type Response struct {
	raw    *http.Response
	body   []byte
	parsed bool
}

// StatusCode returns the HTTP status code.
func (r *Response) StatusCode() int { return r.raw.StatusCode }

// Status returns the HTTP status line.
func (r *Response) Status() string { return r.raw.Status }

// Header returns the response headers.
func (r *Response) Header() http.Header { return r.raw.Header }

// RawResponse returns the underlying *http.Response. In parsed mode its Body is
// already consumed; use Bytes or Body for the payload.
func (r *Response) RawResponse() *http.Response { return r.raw }

// IsSuccess reports whether the status is in the 2xx range.
func (r *Response) IsSuccess() bool { return r.raw.StatusCode >= 200 && r.raw.StatusCode < 300 }

// IsError reports whether the status is 400 or greater.
func (r *Response) IsError() bool { return r.raw.StatusCode >= 400 }

// Body returns a reader over the response body. In parsed mode it reads the
// buffered copy; under SetDoNotParseResponse it returns the live body, which the
// caller must Close.
func (r *Response) Body() io.ReadCloser {
	if r.parsed {
		return io.NopCloser(bytes.NewReader(r.body))
	}
	return r.raw.Body
}

// Bytes returns the buffered response body (empty under SetDoNotParseResponse).
func (r *Response) Bytes() []byte { return r.body }

// String returns the buffered response body as a string.
func (r *Response) String() string { return string(r.body) }

// Decode JSON-decodes the buffered body into v.
func (r *Response) Decode(v any) error {
	if err := json.Unmarshal(r.body, v); err != nil {
		return errors.PropagateAs(errors.KindSystemError, err, "failed to decode response body")
	}
	return nil
}

// Close closes the underlying body. It is a no-op in parsed mode (the body is
// already drained) and is safe to call more than once.
func (r *Response) Close() error {
	if r.parsed || r.raw == nil || r.raw.Body == nil {
		return nil
	}
	return r.raw.Body.Close()
}
