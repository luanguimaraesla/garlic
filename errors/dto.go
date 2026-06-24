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

// NewDTO returns the transferable error DTO for err. Non-garlic errors are
// converted to a generic KindError DTO.
func NewDTO(err error) *DTO {
	e, ok := err.(Transferable)
	if !ok {
		e = Raw(KindError, err.Error())
	}

	return e.ErrorDTO()
}

// MustDecode converts the DTO into an ErrorT and panics when Code is unknown.
func (dto *DTO) MustDecode() *ErrorT {
	return &ErrorT{
		kind:    GetByCode(dto.Code),
		message: dto.Error,
		Details: dto.Details,
	}
}

// Decode converts the DTO into an ErrorT when Code is registered.
func (dto *DTO) Decode() (*ErrorT, bool) {
	kind, ok := LookupByCode(dto.Code)
	if !ok {
		return nil, false
	}

	return &ErrorT{
		kind:    kind,
		message: dto.Error,
		Details: dto.Details,
	}, true
}

// JSON marshals the DTO and panics if the payload cannot be encoded.
func (dto *DTO) JSON() json.RawMessage {
	b, err := json.Marshal(dto)
	if err != nil {
		panic(err)
	}

	return json.RawMessage(b)
}
