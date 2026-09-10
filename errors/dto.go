package errors

import (
	"encoding/json"
)

type Transferable interface {
	ErrorDTO() *DTO
}

type DTO struct {
	Name  string `json:"name,omitempty" mapstructure:"name,omitempty"`
	Error string `json:"error" mapstructure:"error"`
	Code  string `json:"kind" mapstructure:"kind"`

	// Origin carries the kind code of the error that was suppressed before this
	// DTO was built. It is set when a sanitized error (a system failure projected
	// to a generic kind) still needs to tell the client which underlying kind
	// actually failed, so support can trace it. Only the code travels; the
	// suppressed error's message, name, and details never do.
	Origin  string         `json:"origin,omitempty" mapstructure:"origin,omitempty"`
	Details map[string]any `json:"details,omitempty" mapstructure:"details,omitempty"`
}

// NewDTO returns the transferable error DTO for err, or nil when err is nil.
// Non-garlic errors are converted to a generic KindError DTO.
func NewDTO(err error) *DTO {
	if err == nil {
		return nil
	}

	e, ok := err.(Transferable)
	if !ok {
		e = Raw(KindError, err.Error())
	}

	return e.ErrorDTO()
}

// MustDecode converts the DTO into an ErrorT and panics when Code is unknown.
func (dto *DTO) MustDecode() *ErrorT {
	e := newErrorT(GetByCode(dto.Code), dto.Error, NewHeadlessError(dto.Origin))
	e.Details = dto.Details

	return e
}

// Decode converts the DTO into an ErrorT when Code is registered.
func (dto *DTO) Decode() (*ErrorT, bool) {
	kind, ok := LookupByCode(dto.Code)
	if !ok {
		return nil, false
	}

	e := newErrorT(kind, dto.Error, NewHeadlessError(dto.Origin))
	e.Details = dto.Details
	return e, true
}

// DecodeFor calls Decode, then applies a positive, differing status to a kind copy.
// All other decoded data remains unchanged.
func (dto *DTO) DecodeFor(statusCode int) (*ErrorT, bool) {
	e, ok := dto.Decode()
	if !ok {
		return nil, false
	}

	if statusCode > 0 && e.kind.StatusCode() != statusCode {
		e.kind = e.kind.CustomizeStatusCode(statusCode)
	}

	return e, true
}

// JSON marshals the DTO and panics if the payload cannot be encoded.
func (dto *DTO) JSON() json.RawMessage {
	b, err := json.Marshal(dto)
	if err != nil {
		panic(err)
	}

	return json.RawMessage(b)
}
