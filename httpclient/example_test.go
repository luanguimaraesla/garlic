package httpclient_test

import (
	"context"
	"net/http"

	"github.com/luanguimaraesla/garlic/httpclient"
)

func ExampleConnector_Request() {
	conn := httpclient.NewConnector(&httpclient.Config{
		URL: "https://api.example.com",
	})

	type User struct {
		Name string `json:"name"`
	}

	var user User
	err := conn.Request(context.Background(), &httpclient.Request{
		Method:      http.MethodGet,
		URI:         "/users/123",
		QueryParams: map[string]string{"fields": "name"},
	}, &user)
	_ = err
}
