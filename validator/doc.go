// Package validator provides a singleton wrapper around go-playground/validator
// with support for custom field validators and JSON-friendly error messages.
//
// # Initialization
//
// Call [Init] at startup to register custom validators:
//
//	validator.Init(
//	    validator.NewValidation("is_positive", func(fl validator.Field) bool {
//	        return fl.Field().Int() > 0
//	    }),
//	)
//
// If [Init] is not called, [Global] lazily initializes with the built-in
// validators (is_safe_path, alpha_space). Calling [Init] more than once panics.
//
// # Validation
//
// Use [Global] to validate structs:
//
//	type CreateUser struct {
//	    Name  string `json:"name" validate:"required,alpha_space"`
//	    Email string `json:"email" validate:"required,email"`
//	}
//	err := validator.Global().Struct(form)
//
// # Custom Validators
//
// Implement [FieldValidator] or use [NewValidation] for simple cases:
//
//	var positive = validator.NewValidation("is_positive", func(fl validator.Field) bool {
//	    return fl.Field().Int() > 0
//	})
//
// Pass custom validators to [Init] or call [Validator.Extend] on an instance.
//
// # Error Parsing
//
// [ParseValidationErrors] converts go-playground/validator errors into an
// [ErrorT] with per-field hints using JSON tag names for user-friendly messages:
//
//	if err := validator.Global().Struct(form); err != nil {
//	    return validator.ParseValidationErrors(err) // returns KindValidationError
//	}
package validator
