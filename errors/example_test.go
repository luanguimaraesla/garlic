package errors_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
)

func ExampleNew() {
	err := errors.New(errors.KindInvalidRequestError, "email is invalid")
	fmt.Println(err.Error())
	fmt.Println(err.Kind().Name)
	// Output:
	// email is invalid
	// InvalidRequestError
}

func ExamplePropagate() {
	cause := errors.New(errors.KindNotFoundError, "record missing")
	err := errors.Propagate(cause, "failed to fetch user")
	fmt.Println(err.Error())
	fmt.Println(err.Kind().Name)
	// Output:
	// failed to fetch user: record missing
	// NotFoundError
}

func ExamplePropagateAs() {
	cause := fmt.Errorf("connection refused")
	err := errors.PropagateAs(errors.KindSystemError, cause, "database unavailable")
	fmt.Println(err.Error())
	fmt.Println(err.Kind().StatusCode())
	// Output:
	// database unavailable: connection refused
	// 500
}

func ExampleKind_Is() {
	fmt.Println(errors.KindInvalidRequestError.Is(errors.KindUserError))
	fmt.Println(errors.KindInvalidRequestError.Is(errors.KindSystemError))
	// Output:
	// true
	// false
}

func ExampleIsKind() {
	err := errors.New(errors.KindInvalidRequestError, "bad input")
	fmt.Println(errors.IsKind(err, errors.KindUserError))
	fmt.Println(errors.IsKind(err, errors.KindSystemError))
	// Output:
	// true
	// false
}

func ExampleTemplate() {
	tmpl := errors.Template(
		errors.KindNotFoundError,
		"resource not found",
	)

	err := tmpl.New()
	fmt.Println(err.Error())
	fmt.Println(err.Kind().Name)
	// Output:
	// resource not found
	// NotFoundError
}

func ExampleRegister() {
	kind := &errors.Kind{
		Name:           "RateLimitError",
		Code:           "RL0001",
		Description:    "Too many requests",
		HTTPStatusCode: http.StatusTooManyRequests,
		Parent:         errors.KindUserError,
	}
	errors.Register(kind)

	retrieved := errors.GetByCode("RL0001")
	fmt.Println(retrieved.Name)
	fmt.Println(retrieved.StatusCode())
	// Output:
	// RateLimitError
	// 429
}

func ExampleErrorT_ErrorDTO() {
	err := errors.New(errors.KindInvalidRequestError, "email is invalid",
		errors.Hint("provide a valid email address"),
	)
	dto := err.ErrorDTO()
	b, _ := json.Marshal(dto)
	fmt.Println(string(b))
	// Output:
	// {"name":"InvalidRequestError::HTTP400Error::UserError::Error","error":"email is invalid","kind":"C00001","details":{"hint":"provide a valid email address"}}
}
