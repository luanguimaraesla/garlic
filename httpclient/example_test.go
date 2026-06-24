package httpclient_test

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/httpclient"
)

// ExampleClient shows the JSON CRUD path: build one client, fork a per-call
// request, and decode the result.
func ExampleClient() {
	conn, err := httpclient.New(&httpclient.Config{BaseURL: "https://api.example.com"})
	if err != nil {
		return
	}

	type User struct {
		Name string `json:"name"`
	}

	var user User
	_, err = conn.R(context.Background()).SetResult(&user).Get("/users/123")
	_ = err
}

// ExampleClient_auth reads a rotating bearer token from a mounted secret on every
// request.
func ExampleClient_auth() {
	conn, _ := httpclient.New(&httpclient.Config{
		BaseURL:     "https://api.example.com",
		TokenSource: httpclient.FileTokenSource("/var/run/secrets/token"),
	})

	_, _ = conn.R(context.Background()).Get("/me")
}

// ExampleClient_streaming uploads a binary blob with an explicit Content-Length,
// streamed (not buffered) and safe to retry because PUT is idempotent.
func ExampleClient_streaming() {
	conn, _ := httpclient.New(&httpclient.Config{BaseURL: "http://localhost:7233"})

	f, err := os.Open("/tmp/blob")
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	stat, _ := f.Stat()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := conn.R(ctx).
		SetHeader("Content-Type", "application/octet-stream").
		SetBodyStream(f, stat.Size()).
		Put("/files/abc/data")
	if err != nil {
		return
	}
	defer func() { _ = resp.Close() }()

	_ = resp.Header.Get("ETag")
}

// ExampleClient_rawDownload streams a response body straight to a file.
func ExampleClient_rawDownload() {
	conn, _ := httpclient.New(&httpclient.Config{BaseURL: "http://localhost:7233"})

	resp, err := conn.R(context.Background()).Get("/files/abc/data")
	if err != nil {
		return
	}
	defer func() { _ = resp.Close() }()

	dst, _ := os.Create("/tmp/out")
	defer func() { _ = dst.Close() }()
	_, _ = io.Copy(dst, resp.Body)
}

// ExampleClient_typedError decodes a garlic error from a non-2xx response.
func ExampleClient_typedError() {
	conn, _ := httpclient.New(&httpclient.Config{BaseURL: "https://api.example.com"})

	resp, err := conn.R(context.Background()).Get("/users/999")
	if err != nil {
		return
	}
	defer func() { _ = resp.Close() }()

	if resp.IsError() {
		err = resp.DecodeError()
		if errors.IsKind(err, errors.KindNotFoundError) {
			// handle 404
		}
	}
}

// ExampleClient_retry tunes retry: a custom policy and a per-call opt-in for a
// POST that is known to be safe to replay.
func ExampleClient_retry() {
	conn, _ := httpclient.New(&httpclient.Config{
		BaseURL: "https://api.example.com",
		Retry: httpclient.RetryConfig{
			Enabled:    true,
			MaxRetries: 5,
			Policy:     httpclient.DefaultRetryPolicy(),
		},
	})

	_, _ = conn.R(context.Background()).
		EnableRetry().
		SetBodyJSON(map[string]string{"idempotency_key": "abc"}).
		Post("/payments")
}
