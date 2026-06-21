package errors

import (
	"encoding/json"
)

type Transferable interface {
	ErrorDTO() *DTO
}

type DTO struct {
	Name    string         `json:"name,omitempty" mapstructure:"name,omitempty"`
	Error   string         `json:"error" mapstructure:"error"`
	Code    string         `json:"kind" mapstructure:"kind"`
	Details map[string]any `json:"details,omitempty" mapstructure:"details,omitempty"`
}

func NewDTO(err error) *DTO {
	e, ok := err.(Transferable)
	if !ok {
		e = Raw(KindError, err.Error())
	}

	return e.ErrorDTO()
}

func (dto *DTO) Decode() *ErrorT {
	return &ErrorT{
		kind:    GetByCode(dto.Code),
		message: dto.Error,
		Details: dto.Details,
	}
}

// DecodeSafe reconstructs an *ErrorT from a DTO received over the wire without
// panicking on an unknown or empty kind code. When the code is registered the
// decode is faithful (the original kind, message, and details are preserved).
// Otherwise the error is classified with fallback (or KindSystemError when
// fallback is nil) and fallbackOpts are applied; fallbackOpts are ignored on
// the faithful path. Use it instead of Decode for untrusted response bodies.
func (dto *DTO) DecodeSafe(fallback *Kind, fallbackOpts ...Opt) *ErrorT {
	if kind, ok := LookupByCode(dto.Code); ok {
		details := dto.Details
		if details == nil {
			details = map[string]any{}
		}

		return &ErrorT{
			kind:    kind,
			message: dto.Error,
			Details: details,
		}
	}

	if fallback == nil {
		fallback = KindSystemError
	}

	return New(fallback, dto.Error, fallbackOpts...)
}

// JSON serializes the DTO struct into a JSON formatted byte slice.
// It returns the serialized data as json.RawMessage, which is a type alias for []byte.
// If an error occurs during the marshaling process, the function will panic.
func (dto *DTO) JSON() json.RawMessage {
	b, err := json.Marshal(dto)
	if err != nil {
		panic(err)
	}

	return json.RawMessage(b)
}
