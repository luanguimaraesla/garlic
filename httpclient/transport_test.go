//go:build unit

package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

func TestTransport_connectionReused(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	})

	doRequest := func() bool {
		var reused bool
		trace := &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
		}
		ctx := httptrace.WithClientTrace(context.Background(), trace)
		if _, err := c.R(ctx).Get("/x"); err != nil {
			t.Fatal(err)
		}
		return reused
	}

	doRequest() // primes the pool
	if !doRequest() {
		t.Error("expected the second request to reuse the pooled connection")
	}
}

func TestTransport_errorIsTyped(t *testing.T) {
	rt := RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial failed")
	})
	c := newClientWithTransport(t, rt, &Config{Retry: RetryConfig{}})

	_, err := c.R(context.Background()).Get("/x")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.IsKind(err, errors.KindSystemError) {
		t.Errorf("want a system error, got %v", err)
	}
}

func TestTransport_checkRedirectHonored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	c, _ := New(&Config{
		BaseURL:       srv.URL,
		Retry:         RetryConfig{},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})

	_, err := c.R(context.Background()).Get("/redirect")
	var re *ResponseError
	if !errors.As(err, &re) {
		t.Fatalf("expected a ResponseError, got %v", err)
	}
	if re.StatusCode() != http.StatusFound {
		t.Errorf("status = %d, want 302 (redirect not followed)", re.StatusCode())
	}
}
