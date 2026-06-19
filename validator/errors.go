package validator

import (
	"github.com/luanguimaraesla/garlic/errors"
)

// KindValidationError classifies a form or field that the user filled in
// incorrectly. It descends from errors.KindInvalidRequestError; importing this
// package registers it with the errors registry.
var KindValidationError = &errors.Kind{
	Name:        "ValidationError",
	Code:        "C00002",
	Description: "Some field on a form was filled incorrectly by the user or is missing.",
	Parent:      errors.KindInvalidRequestError,
}

func init() {
	errors.Register(KindValidationError)
}
