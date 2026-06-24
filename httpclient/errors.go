package httpclient

import (
	"github.com/luanguimaraesla/garlic/errors"
)

var (
	// KindResponseDecodeError classifies failures to decode response bodies.
	KindResponseDecodeError = &errors.Kind{
		Name:        "ResponseDecodeError",
		Code:        "C10001",
		Description: "The request was executed correctly but the response could not be decoded.",
		Parent:      errors.KindSystemError,
	}

	// KindUnknownResponseError classifies error responses that are not valid
	// garlic error DTOs.
	KindUnknownResponseError = &errors.Kind{
		Name:        "UnknownResponseError",
		Code:        "C10003",
		Description: "The request was executed correctly but the response is unknown.",
		Parent:      errors.KindSystemError,
	}
)

func init() {
	errors.Register(
		KindResponseDecodeError,
		KindUnknownResponseError,
	)
}
