//go:build unit

package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestDefaultRetryPolicy_CheckRetry(t *testing.T) {
	p := DefaultRetryPolicy()
	ctx := context.Background()

	cases := []struct {
		name string
		resp *http.Response
		err  error
		want bool
	}{
		{"transport error", nil, fmt.Errorf("boom"), true},
		{"nil resp no err", nil, nil, false},
		{"500", textResponse(http.StatusInternalServerError, "", nil), nil, true},
		{"501 not retried", textResponse(http.StatusNotImplemented, "", nil), nil, false},
		{"503", textResponse(http.StatusServiceUnavailable, "", nil), nil, true},
		{"429", textResponse(http.StatusTooManyRequests, "", nil), nil, true},
		{"404 not retried", textResponse(http.StatusNotFound, "", nil), nil, false},
		{"200 not retried", textResponse(http.StatusOK, "", nil), nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.CheckRetry(ctx, http.MethodGet, tc.resp, tc.err, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("CheckRetry = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDefaultRetryPolicy_CheckRetry_contextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok, err := DefaultRetryPolicy().CheckRetry(ctx, http.MethodGet, nil, nil, 0)
	if ok {
		t.Error("a cancelled context must stop retrying")
	}
	if err == nil {
		t.Error("a cancelled context should return the context error")
	}
}

func TestDefaultRetryPolicy_Backoff(t *testing.T) {
	p := DefaultRetryPolicy()

	header := http.Header{}
	header.Set("Retry-After", "1")
	if d := p.Backoff(time.Millisecond, time.Minute, 0, textResponse(http.StatusServiceUnavailable, "", header)); d != time.Second {
		t.Errorf("Retry-After backoff = %v, want 1s", d)
	}

	d := p.Backoff(10*time.Millisecond, 100*time.Millisecond, 5, nil)
	if d < 10*time.Millisecond || d > 100*time.Millisecond {
		t.Errorf("exponential backoff out of bounds: %v", d)
	}
}

func TestIsIdempotent(t *testing.T) {
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete, http.MethodTrace} {
		if !IsIdempotent(m) {
			t.Errorf("%s should be idempotent", m)
		}
	}
	for _, m := range []string{http.MethodPost, http.MethodPatch} {
		if IsIdempotent(m) {
			t.Errorf("%s should not be idempotent", m)
		}
	}
}
