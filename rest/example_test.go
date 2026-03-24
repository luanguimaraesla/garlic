package rest_test

import (
	"context"
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/rest"
)

func ExampleGetServer() {
	srv := rest.GetServer("api")
	_ = srv.Listen(context.Background(), ":8080")
}

func ExampleWriteResponse() {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		type User struct {
			Name string `json:"name"`
		}
		rest.WriteResponse(http.StatusOK, User{Name: "Alice"}).Must(w)
		return nil
	}
	_ = handler
}

func ExampleWriteError() {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		err := errors.New(errors.KindNotFoundError, "user not found",
			errors.Hint("check the user ID and try again"),
		)
		// WriteError maps the error kind to the correct HTTP status (404)
		// and serialises the ErrorDTO as JSON.
		rest.WriteError(err).Must(w)
		return nil
	}
	_ = handler
}
