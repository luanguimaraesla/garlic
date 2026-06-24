//go:build unit

package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/luanguimaraesla/garlic/errors"
)

func TestResponse_DecodeErrorReturnsGarlicError(t *testing.T) {
	raw, _ := json.Marshal(errors.New(errors.KindNotFoundError, "missing").ErrorDTO())
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusNotFound, string(raw), nil), nil
	}), &Config{Retry: RetryConfig{}})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	gerr := resp.DecodeError()
	if !errors.IsKind(gerr, errors.KindNotFoundError) {
		t.Fatalf("decoded error kind = %v, want KindNotFoundError", gerr)
	}
	if gerr.Error() != "missing" {
		t.Errorf("message = %q, want missing", gerr.Error())
	}
}

func TestResponse_DecodeErrorRejectsUnknownKind(t *testing.T) {
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusBadGateway, `{"error":"weird","kind":"ZZZ999"}`, nil), nil
	}), &Config{Retry: RetryConfig{}})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	gerr := resp.DecodeError()
	if !errors.IsKind(gerr, KindUnknownResponseError) {
		t.Fatalf("decoded error kind = %v, want KindUnknownResponseError", gerr)
	}
}

func TestResponse_DecodeErrorRejectsNonErrorResponse(t *testing.T) {
	resp := &Response{Response: textResponse(http.StatusOK, `{}`, nil)}

	gerr := resp.DecodeError()
	if !errors.IsKind(gerr, KindResponseDecodeError) {
		t.Fatalf("decoded error kind = %v, want KindResponseDecodeError", gerr)
	}
}

func TestRequest_SendDoesNotErrorOnHTTPStatus(t *testing.T) {
	raw, _ := json.Marshal(errors.New(errors.KindNotFoundError, "missing").ErrorDTO())
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusNotFound, string(raw), nil), nil
	}), &Config{Retry: RetryConfig{}})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatalf("Get returned error for HTTP status: %v", err)
	}
	if !resp.IsError() {
		t.Fatal("expected response to report an error status")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d, ok := parseRetryAfter("5"); !ok || d != 5*time.Second {
		t.Errorf("delta-seconds: got %v, %v", d, ok)
	}
	if _, ok := parseRetryAfter(""); ok {
		t.Error("empty should be (0, false)")
	}
	if _, ok := parseRetryAfter("garbage"); ok {
		t.Error("garbage should be (0, false)")
	}
	if _, ok := parseRetryAfter("-3"); ok {
		t.Error("negative should be (0, false)")
	}
	future := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	if d, ok := parseRetryAfter(future); !ok || d <= 0 {
		t.Errorf("HTTP-date: got %v, %v", d, ok)
	}
}
