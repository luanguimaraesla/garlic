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

func TestDecodeErrorBody_dtoKnownCode_faithful(t *testing.T) {
	raw, _ := json.Marshal(errors.New(errors.KindNotFoundError, "missing").ErrorDTO())

	e := decodeErrorBody(raw, http.StatusNotFound, "message")
	if !errors.IsKind(e, errors.KindNotFoundError) {
		t.Error("expected the faithful NotFound kind")
	}
	if len(e.Troubleshooting.Context) != 0 {
		t.Error("a faithful decode must not be marked as a fallback")
	}
}

func TestDecodeErrorBody_noPanicMatrix(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
		field  string
		want   string // expected message
	}{
		{"dto unknown code", `{"name":"X","error":"weird","kind":"ZZZ999"}`, 503, "message", "weird"},
		{"message shape", `{"message":"nope"}`, 400, "message", "nope"},
		{"custom message field", `{"msg":"custom"}`, 400, "msg", "custom"},
		{"plain text", "just text", 500, "message", "just text"},
		{"html", "<html>bad gateway</html>", 502, "message", "<html>bad gateway</html>"},
		{"empty", "", 500, "message", http.StatusText(500)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("decodeErrorBody panicked: %v", r)
				}
			}()

			e := decodeErrorBody([]byte(tc.body), tc.status, tc.field)
			if e.Error() != tc.want {
				t.Errorf("message = %q, want %q", e.Error(), tc.want)
			}
			if e.Kind().StatusCode() != tc.status {
				t.Errorf("kind status = %d, want %d", e.Kind().StatusCode(), tc.status)
			}
		})
	}
}

func TestDecodeErrorBody_fallbackMarkStaysInTroubleshooting(t *testing.T) {
	e := decodeErrorBody([]byte("plain text"), 500, "message")

	if len(e.Troubleshooting.Context) == 0 {
		t.Error("expected a fallback mark in the troubleshooting context")
	}
	dto := e.ErrorDTO()
	if _, leaked := dto.Details["upstream_decode_fallback"]; leaked {
		t.Error("the fallback mark must never appear in the serialized DTO")
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

func TestResponseError_endToEndPreservesStatusRetryAfterHeaders(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", "7")
	header.Set("X-Request-Id", "req-9")

	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusTooManyRequests, `{"message":"slow down"}`, header), nil
	}), &Config{Retry: RetryConfig{}})

	_, err := c.R(context.Background()).Get("/x")

	var re *ResponseError
	if !errors.As(err, &re) {
		t.Fatalf("expected a ResponseError, got %v", err)
	}
	if re.StatusCode() != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", re.StatusCode())
	}
	if d, ok := re.RetryAfter(); !ok || d != 7*time.Second {
		t.Errorf("Retry-After = %v, %v", d, ok)
	}
	if re.Header().Get("X-Request-Id") != "req-9" {
		t.Errorf("preserved header missing: %v", re.Header())
	}
	if !errors.IsKind(err, errors.KindUserError) {
		t.Error("429 should classify as a user-class error")
	}
}
