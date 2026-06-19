package httpclient

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luanguimaraesla/garlic/errors"
)

const maxBodySnippet = 4 << 10 // 4 KiB cap on the retained body snippet.

// preservedHeaders is the small, diagnostic allowlist of response headers
// carried onto a ResponseError. It deliberately excludes sensitive or volatile
// headers such as Set-Cookie and Authorization.
var preservedHeaders = []string{
	"Retry-After",
	"Content-Type",
	"X-Request-Id",
	"X-Session-Id",
	"X-Ratelimit-Remaining",
	"X-Ratelimit-Reset",
}

// ResponseError is the typed error returned when an HTTP response carries a
// non-success status. It wraps a garlic *errors.ErrorT (so errors.IsKind,
// errors.AsKind, and structured logging keep working) and additionally exposes
// the HTTP status, the server-driven Retry-After hint, a selected subset of
// response headers, and a snippet of the raw body for diagnostics.
type ResponseError struct {
	*errors.ErrorT

	retryAfter time.Duration
	hasRetry   bool
	header     http.Header
	body       string
}

// StatusCode returns the HTTP status of the failed response. It is sourced from
// the error's kind, so it always equals Kind().StatusCode() and can never
// disagree with the classification.
func (e *ResponseError) StatusCode() int { return e.Kind().StatusCode() }

// RetryAfter returns the delay parsed from the response's Retry-After header and
// true when the header was present and valid; otherwise it returns 0 and false.
func (e *ResponseError) RetryAfter() (time.Duration, bool) { return e.retryAfter, e.hasRetry }

// Header returns the preserved subset of response headers. It is never nil.
func (e *ResponseError) Header() http.Header { return e.header }

// Body returns the captured (possibly truncated) response body.
func (e *ResponseError) Body() string { return e.body }

// Unwrap exposes the wrapped *errors.ErrorT so errors.As, errors.Is, and the
// kind-classification helpers traverse into it.
func (e *ResponseError) Unwrap() error { return e.ErrorT }

// newResponseError builds a typed *ResponseError from a non-success response's
// status, headers, and already-read body bytes. raw may be nil (for example a
// streaming response left unparsed), in which case the error is classified by
// status alone. It preserves the status, Retry-After, and selected headers, and
// never panics on the body shape.
func newResponseError(status int, header http.Header, raw []byte, messageField string) *ResponseError {
	if len(raw) > maxBodySnippet {
		raw = raw[:maxBodySnippet]
	}

	retry, hasRetry := parseRetryAfter(header.Get("Retry-After"))

	return &ResponseError{
		ErrorT:     decodeErrorBody(raw, status, messageField),
		retryAfter: retry,
		hasRetry:   hasRetry,
		header:     selectHeaders(header),
		body:       string(raw),
	}
}

// decodeErrorBody reconstructs the richest garlic error it can from a response
// body, trying the garlic DTO shape, then the configurable message shape, then
// the raw body. It never panics on an unknown or empty kind code, and marks
// every fallback path in the troubleshooting context (logged, never serialized).
func decodeErrorBody(raw []byte, status int, messageField string) *errors.ErrorT {
	fallback := errors.KindForStatus(status)
	trimmed := bytes.TrimSpace(raw)

	if len(trimmed) > 0 && trimmed[0] == '{' {
		// 1) garlic DTO shape: {"name","error","kind","details"}.
		var dto errors.DTO
		if json.Unmarshal(trimmed, &dto) == nil && dto.Code != "" {
			return dto.DecodeSafe(fallback, fallbackMark("unrecognized garlic kind code "+dto.Code))
		}

		// 2) configurable message shape, e.g. {"message":"..."}.
		var obj map[string]json.RawMessage
		if json.Unmarshal(trimmed, &obj) == nil {
			if rawMsg, ok := obj[messageField]; ok {
				var msg string
				if json.Unmarshal(rawMsg, &msg) == nil && msg != "" {
					return errors.New(fallback, msg, fallbackMark("non-garlic message body"))
				}
			}
		}
	}

	// 3) plain text, HTML, or empty body.
	message := strings.TrimSpace(string(trimmed))
	if message == "" {
		message = http.StatusText(status)
	}

	return errors.New(fallback, message, fallbackMark("unstructured response body"))
}

// fallbackMark records, in the error's troubleshooting context (logged via
// errors.Zap but never serialized into a DTO), that the error was reconstructed
// from a response the client could not faithfully decode as a garlic error.
func fallbackMark(reason string) errors.Opt {
	return errors.Context(errors.Field("upstream_decode_fallback", reason))
}

// parseRetryAfter interprets an RFC 7231 Retry-After value, which is either
// delta-seconds or an HTTP-date.
func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}

	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}

	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d, true
		}
		return 0, true
	}

	return 0, false
}

func selectHeaders(src http.Header) http.Header {
	out := http.Header{}
	for _, name := range preservedHeaders {
		if vals := src.Values(name); len(vals) > 0 {
			out[http.CanonicalHeaderKey(name)] = append([]string(nil), vals...)
		}
	}
	return out
}
