//go:build unit

package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/luanguimaraesla/garlic/tracing"
)

func TestHeaders_requestOverridesBase(t *testing.T) {
	var got http.Header
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		got = req.Header.Clone()
		return textResponse(http.StatusOK, "{}", nil), nil
	})

	c := newClientWithTransport(t, rt, &Config{
		BaseHeaders: map[string]string{"X-App": "garlic", "X-Env": "test"},
	})

	if _, err := c.R(context.Background()).SetHeader("X-Env", "prod").Get("/x"); err != nil {
		t.Fatal(err)
	}
	if got.Get("X-App") != "garlic" {
		t.Errorf("base header lost: %q", got.Get("X-App"))
	}
	if got.Get("X-Env") != "prod" {
		t.Errorf("request header should win: %q", got.Get("X-Env"))
	}
}

func TestTracing_headersPropagatedFromContext(t *testing.T) {
	var got http.Header
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		got = req.Header.Clone()
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	id := uuid.New()
	ctx := tracing.SetContextRequestId(context.Background(), id)
	ctx = tracing.SetContextSessionId(ctx, "sess-123")

	if _, err := c.R(ctx).Get("/x"); err != nil {
		t.Fatal(err)
	}
	if got.Get("X-Request-ID") != id.String() {
		t.Errorf("X-Request-ID = %q", got.Get("X-Request-ID"))
	}
	if got.Get("X-Session-ID") != "sess-123" {
		t.Errorf("X-Session-ID = %q", got.Get("X-Session-ID"))
	}
}

func TestAuth_staticToken(t *testing.T) {
	var auth string
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		auth = req.Header.Get("Authorization")
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, &Config{TokenSource: StaticToken("secret")})

	if _, err := c.R(context.Background()).Get("/x"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer secret" {
		t.Errorf("Authorization = %q", auth)
	}
}

func TestAuth_requestTokenOverridesSource(t *testing.T) {
	var auth string
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		auth = req.Header.Get("Authorization")
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, &Config{TokenSource: StaticToken("client-tok")})

	if _, err := c.R(context.Background()).SetAuthToken("req-tok").Get("/x"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer req-tok" {
		t.Errorf("per-request token should win: %q", auth)
	}
}

func TestAuth_freshTokenPerRetryAttempt(t *testing.T) {
	counter := 0
	src := TokenSourceFunc(func() (string, error) {
		counter++
		return fmt.Sprintf("tok-%d", counter), nil
	})

	attempt := 0
	var authHeaders []string
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		authHeaders = append(authHeaders, req.Header.Get("Authorization"))
		attempt++
		if attempt == 1 {
			return textResponse(http.StatusServiceUnavailable, "", nil), nil
		}
		return textResponse(http.StatusOK, "{}", nil), nil
	})

	c := newClientWithTransport(t, rt, &Config{TokenSource: src, Retry: fastRetry(2)})
	if _, err := c.R(context.Background()).Get("/x"); err != nil {
		t.Fatal(err)
	}
	if len(authHeaders) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(authHeaders))
	}
	if authHeaders[0] != "Bearer tok-1" || authHeaders[1] != "Bearer tok-2" {
		t.Errorf("expected a fresh token per attempt, got %v", authHeaders)
	}
}

func TestAuth_tokenErrorAbortsBeforeRequest(t *testing.T) {
	called := false
	rt := RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	src := TokenSourceFunc(func() (string, error) { return "", fmt.Errorf("token unavailable") })

	c := newClientWithTransport(t, rt, &Config{TokenSource: src})
	_, err := c.R(context.Background()).Get("/x")
	if err == nil {
		t.Fatal("expected an error when the token source fails")
	}
	if called {
		t.Error("no request should be sent when the token source fails")
	}
}

func TestFileTokenSource_readsAndTrims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  abc123\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tok, err := FileTokenSource(path).Token()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "abc123" {
		t.Errorf("token = %q, want abc123", tok)
	}
}

func TestFileTokenSource_missingFile(t *testing.T) {
	if _, err := FileTokenSource("/nonexistent/path/token").Token(); err == nil {
		t.Fatal("expected an error for a missing token file")
	}
}
