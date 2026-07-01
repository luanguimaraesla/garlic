package rest

import (
	"encoding/json"
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
)

type PayloadMessage struct {
	Message string `json:"message"`
}

type Response struct {
	StatusCode int
	Payload    any
}

// unknownErrorResponse is returned when WriteError is called with a nil error.
// It is projected through protectErrorDTO so it carries no sensitive detail.
var unknownErrorResponse = WriteResponse(
	http.StatusInternalServerError,
	protectErrorDTO(errors.Raw(errors.KindSystemError, "unknown error")),
)

func (r *Response) Must(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.StatusCode)
	if err := json.NewEncoder(w).Encode(r.Payload); err != nil {
		// Last-ditch fallback, in the canonical error envelope.
		http.Error(w,
			`{"error":"Internal Server Error","kind":"P00002"}`,
			http.StatusInternalServerError,
		)
	}
}

// WriteResponse is a generic function to create a response with a payload
func WriteResponse(statusCode int, payload any) *Response {
	return &Response{
		StatusCode: statusCode,
		Payload:    payload,
	}
}

// WriteMessage is a helper function to create a response with a message
func WriteMessage(statusCode int, message string) *Response {
	return WriteResponse(statusCode, PayloadMessage{Message: message})
}

// WriteError converts an error into a canonical error response. It is the one
// blessed path for error responses: the status comes from the error's kind, and
// the body is projected through protectErrorDTO so user errors are exposed in
// full while system errors are sanitized to their generic HTTP-status kind, with
// the real kind code kept only as an origin reference. A nil or non-garlic error
// is treated as an opaque internal failure.
func WriteError(err error) *Response {
	if err == nil {
		return unknownErrorResponse
	}

	var e *errors.ErrorT
	if !errors.As(err, &e) {
		// A non-garlic error carries no kind; treat it as an opaque internal
		// failure so nothing sensitive leaks.
		e = errors.Raw(errors.KindSystemError, "internal server error")
	}

	return WriteResponse(e.Kind().StatusCode(), protectErrorDTO(e))
}

// protectErrorDTO builds the wire-safe body for an error. User-class errors are
// safe to expose and cross the wire in full. Every other error is replaced by
// the generic kind for its HTTP status, so its name, dynamic message, and
// details stay server-side; the real error is kept as a sanitized origin, so its
// kind code still reaches the client for troubleshooting, alongside a hint to
// contact support.
func protectErrorDTO(err *errors.ErrorT) *errors.DTO {
	// Classify by the error's own kind, not its cause chain. The HTTP status also
	// comes from this kind, so keying the redaction off IsKind (which walks the
	// Unwrap chain) would let a system error that merely wraps a user-kinded cause
	// be served in full under a 500, leaking its message and details.
	if !err.Kind().Is(errors.KindUserError) {
		generic := errors.KindForStatus(err.Kind().StatusCode())
		err = errors.MirrorOverride(generic, err, errors.Hint("Suppressed system error, contact the support"))
	}

	return err.ErrorDTO()
}
