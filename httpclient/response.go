package httpclient

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
)

// Response wraps the raw HTTP response returned by the upstream service. The
// body stays open unless a helper such as Decode or DecodeError consumes it.
type Response struct {
	*http.Response
}

// IsSuccess reports whether the status is in the 2xx range.
func (r *Response) IsSuccess() bool { return r.StatusCode >= 200 && r.StatusCode < 300 }

// IsError reports whether the status is 400 or greater.
func (r *Response) IsError() bool { return r.StatusCode >= 400 }

// Decode JSON-decodes the response body into v and closes it.
func (r *Response) Decode(v any) error {
	if r == nil || r.Response == nil || r.Body == nil {
		return errors.New(KindResponseDecodeError, "response body is empty")
	}
	defer r.drainAndClose()

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return errors.PropagateAs(KindResponseDecodeError, err, "failed to decode response body")
	}

	return nil
}

// DecodeError decodes a garlic error DTO from an error response body and closes
// it. Callers can use errors.IsKind on the returned error.
func (r *Response) DecodeError() error {
	if r == nil || r.Response == nil || r.Body == nil {
		return errors.New(KindResponseDecodeError, "response body is empty")
	}
	if !r.IsError() {
		return errors.New(KindResponseDecodeError, "failed to decode a non-error response into error")
	}

	defer r.drainAndClose()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var gerrDTO errors.DTO
	if err := dec.Decode(&gerrDTO); err != nil {
		return errors.PropagateAs(KindUnknownResponseError, err, "failed to decode response error")
	}

	gerr, ok := gerrDTO.Decode()
	if !ok {
		return errors.New(KindUnknownResponseError, "failed to parse error code", errors.Context(
			errors.Field("code", gerrDTO.Code),
			errors.Field("http_status", r.StatusCode),
		))
	}

	return gerr
}

// Close closes the response body. It is safe to call more than once.
func (r *Response) Close() error {
	if r == nil || r.Response == nil || r.Body == nil {
		return nil
	}

	body := r.Body
	r.Body = nil
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4<<10))
	return body.Close()
}

func (r *Response) drainAndClose() {
	drainAndClose(r.Body)
	r.Body = nil
}

// drainAndClose reads a bounded tail of the body before closing it so idle
// connections can usually return to the pool.
func drainAndClose(rc io.ReadCloser) {
	if rc == nil {
		return
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 4<<10))
	_ = rc.Close()
}
