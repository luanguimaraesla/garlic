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

	// KindUnknownResponseError classifies a recognizable garlic error DTO whose
	// kind is not registered in this program, which usually means the peer runs a
	// garlic version or a kind set this one does not know.
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
