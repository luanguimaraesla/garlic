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
// It is projected through PublicDTO so it carries no sensitive detail.
var unknownErrorResponse = WriteResponse(
	http.StatusInternalServerError,
	errors.Raw(errors.KindSystemError, "unknown error").PublicDTO(),
)

func (r *Response) Must(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.StatusCode)
	if err := json.NewEncoder(w).Encode(r.Payload); err != nil {
		// Last-ditch fallback, in the canonical error envelope.
		http.Error(w,
			`{"name":"SystemError::Error","error":"Any error that was caused by some unexpected system failure.","kind":"P00002"}`,
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
// the body is projected through [errors.ErrorT.PublicDTO] so user errors are
// exposed in full while system errors are genericized to their HTTP status (only
// a per-status code and the standard status text cross the wire). A nil or
// non-garlic error is treated as an opaque internal failure.
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

	return WriteResponse(e.Kind().StatusCode(), e.PublicDTO())
}
