// Package request provides helpers for parsing HTTP request parameters, query
// strings, and JSON bodies with integrated error handling.
//
// # Path Parameters
//
// Extract typed values from Chi URL parameters:
//
//	id, err := request.ParseResourceUUID(r, "id")
//	page, err := request.ParseResourceInt(r, "page")
//	slug, err := request.ParseResourceString(r, "slug")
//
// Parsing errors are returned as user-friendly validation errors with hints
// describing the expected format.
//
// # Query Parameters
//
//	limit, start := request.ParseParamPagination(r)
//	uid, err := request.ParseParamUUID(r, "user_id")
//	uid, err := request.ParseOptionalParamUUID(r, "user_id")
//	name, err := request.ParseParamString(r, "name")
//	ok, err := request.ParseOptionalParamBool(r, "active")
//
// # Body Decoding
//
// [DecodeRequestBody] reads and JSON-decodes the request body into a struct.
// [ValidateForm] runs go-playground/validator rules. [ParseForm] combines both
// steps and calls the [Form] interface's ToModel method for domain conversion:
//
//	type CreateUserForm struct {
//	    Name  string `json:"name" validate:"required"`
//	    Email string `json:"email" validate:"required,email"`
//	}
//	func (f *CreateUserForm) ToModel() (User, error) { ... }
//
//	user, err := request.ParseForm[User](r, &CreateUserForm{})
//
// [ParseUnsafeForm] accepts an [UnsafeForm] that receives a [crypto.Manager]
// for decrypting sensitive fields during model conversion.
package request
