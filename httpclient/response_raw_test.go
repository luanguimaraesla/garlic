//go:build unit

package httpclient

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

func TestResponse_BodyStaysOpenByDefault(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("streamed-bytes"))
	})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Close() }()

	b, _ := io.ReadAll(resp.Body)
	if string(b) != "streamed-bytes" {
		t.Errorf("body = %s", b)
	}
}

func TestResponse_DecodeConsumesBody(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"bob"}`))
	})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}

	var out struct {
		Name string `json:"name"`
	}
	if err := resp.Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "bob" {
		t.Errorf("decoded name = %q", out.Name)
	}
	if resp.Body != nil {
		t.Error("Decode should close the response body")
	}
}

func TestResponse_CloseIsIdempotent(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := resp.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestResponse_DecodeMalformedJSONResult(t *testing.T) {
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, "not json", nil), nil
	}), nil)

	var out struct {
		Name string `json:"name"`
	}
	_, err := c.R(context.Background()).SetResult(&out).Get("/x")
	if !errors.IsKind(err, KindResponseDecodeError) {
		t.Fatalf("error kind = %v, want KindResponseDecodeError", err)
	}
}
