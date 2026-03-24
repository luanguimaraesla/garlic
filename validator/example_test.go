package validator_test

import (
	"github.com/luanguimaraesla/garlic/validator"
)

func ExampleGlobal() {
	type CreateUser struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	form := CreateUser{Name: "", Email: "not-an-email"}
	if err := validator.Global().Struct(form); err != nil {
		validationErr := validator.ParseValidationErrors(err)
		_ = validationErr // returns KindValidationError with per-field hints
	}
}
