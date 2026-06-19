//go:build unit

package httpclient

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestResponse_doNotParseStreamsBody(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("streamed-bytes"))
	})

	resp, err := c.R(context.Background()).SetDoNotParseResponse(true).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Close() }()

	if len(resp.Bytes()) != 0 {
		t.Error("body should not be buffered in no-parse mode")
	}
	b, _ := io.ReadAll(resp.Body())
	if string(b) != "streamed-bytes" {
		t.Errorf("streamed body = %s", b)
	}
}

func TestResponse_parsedBytesAndDecode(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"bob"}`))
	})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Bytes()) != `{"name":"bob"}` {
		t.Errorf("bytes = %s", resp.Bytes())
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
}

func TestResponse_closeIsIdempotent(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resp, err := c.R(context.Background()).SetDoNotParseResponse(true).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	_ = resp.Close() // second close must be safe
}

func TestResponse_decodeMalformedJSONResult(t *testing.T) {
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, "not json", nil), nil
	}), nil)

	var out struct {
		Name string `json:"name"`
	}
	_, err := c.R(context.Background()).SetResult(&out).Get("/x")
	if err == nil {
		t.Fatal("expected a decode error for a non-JSON body")
	}
}
