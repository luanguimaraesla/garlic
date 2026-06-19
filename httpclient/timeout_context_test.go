//go:build unit

package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func slowServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTimeout_configDeadlineEnforced(t *testing.T) {
	srv := slowServer(t)
	c, _ := New(&Config{BaseURL: srv.URL, Timeout: 50 * time.Millisecond, Retry: RetryConfig{}})

	start := time.Now()
	_, err := c.R(context.Background()).Get("/slow")
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("timeout not enforced, took %v", elapsed)
	}
}

func TestTimeout_perCallOverride(t *testing.T) {
	srv := slowServer(t)
	c, _ := New(&Config{BaseURL: srv.URL, Timeout: 5 * time.Second, Retry: RetryConfig{}})

	start := time.Now()
	_, err := c.R(context.Background()).SetTimeout(50 * time.Millisecond).Get("/slow")
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("per-call timeout not applied, took %v", elapsed)
	}
}

func TestRetry_cancellationInterruptsBackoff(t *testing.T) {
	crt := &countingRoundTripper{
		respond: func(int) (*http.Response, error) {
			return textResponse(http.StatusServiceUnavailable, "", nil), nil
		},
	}
	c := newClientWithTransport(t, crt.RoundTrip, &Config{
		Retry: RetryConfig{
			Enabled:    true,
			MaxRetries: 10,
			MinWait:    500 * time.Millisecond,
			MaxWait:    time.Second,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := c.R(ctx).Get("/x")
	if err == nil {
		t.Fatal("expected a cancellation error")
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("backoff was not interrupted by cancellation, took %v", elapsed)
	}
	if crt.count() > 2 {
		t.Errorf("too many attempts after cancellation: %d", crt.count())
	}
}
