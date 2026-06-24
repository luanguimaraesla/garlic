//go:build unit

package httpclient

import (
	"context"
	"net/http"
	"testing"
)

func alwaysStatus(status int) *countingRoundTripper {
	return &countingRoundTripper{
		respond: func(int) (*http.Response, error) {
			return textResponse(status, "", nil), nil
		},
	}
}

func failThenOK(failures, status int) *countingRoundTripper {
	return &countingRoundTripper{
		respond: func(attempt int) (*http.Response, error) {
			if attempt < failures {
				return textResponse(status, "", nil), nil
			}
			return textResponse(http.StatusOK, "{}", nil), nil
		},
	}
}

func TestRetry_postNotRetriedByDefault(t *testing.T) {
	crt := alwaysStatus(http.StatusServiceUnavailable)
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(3)})

	resp, err := c.R(context.Background()).SetBodyBytes([]byte("x")).Post("/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if crt.count() != 1 {
		t.Errorf("POST must not retry by default, attempts = %d", crt.count())
	}
}

func TestRetry_enableRetryAllowsPost(t *testing.T) {
	crt := failThenOK(1, http.StatusServiceUnavailable)
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(3)})

	resp, err := c.R(context.Background()).EnableRetry().SetBodyBytes([]byte("x")).Post("/x")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsSuccess() {
		t.Error("expected success after retry")
	}
	if crt.count() != 2 {
		t.Errorf("attempts = %d, want 2", crt.count())
	}
}

func TestRetry_getRetriedUntilSuccess(t *testing.T) {
	crt := failThenOK(2, http.StatusServiceUnavailable)
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(5)})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsSuccess() {
		t.Error("expected success")
	}
	if crt.count() != 3 {
		t.Errorf("attempts = %d, want 3", crt.count())
	}
}

func TestRetry_disabledPerCall(t *testing.T) {
	crt := alwaysStatus(http.StatusServiceUnavailable)
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(3)})

	resp, err := c.R(context.Background()).DisableRetry().Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if crt.count() != 1 {
		t.Errorf("DisableRetry should prevent retries, attempts = %d", crt.count())
	}
}

func TestRetry_maxRetriesExact(t *testing.T) {
	crt := alwaysStatus(http.StatusServiceUnavailable)
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(2)})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if crt.count() != 3 { // 1 initial + 2 retries
		t.Errorf("attempts = %d, want 3", crt.count())
	}
}

func TestRetry_4xxNotRetried(t *testing.T) {
	crt := alwaysStatus(http.StatusNotFound)
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(3)})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if crt.count() != 1 {
		t.Errorf("a 404 must not be retried, attempts = %d", crt.count())
	}
}

func TestRetry_429HonorsRetryAfter(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", "0")
	crt := &countingRoundTripper{
		respond: func(attempt int) (*http.Response, error) {
			if attempt == 0 {
				return textResponse(http.StatusTooManyRequests, "", header), nil
			}
			return textResponse(http.StatusOK, "{}", nil), nil
		},
	}
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(2)})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsSuccess() {
		t.Error("expected success after honoring Retry-After")
	}
	if crt.count() != 2 {
		t.Errorf("attempts = %d, want 2", crt.count())
	}
}

func TestRetry_transportErrorRetried(t *testing.T) {
	crt := &countingRoundTripper{
		respond: func(attempt int) (*http.Response, error) {
			if attempt == 0 {
				return nil, context.DeadlineExceeded
			}
			return textResponse(http.StatusOK, "{}", nil), nil
		},
	}
	c := newClientWithTransport(t, crt.RoundTrip, &Config{Retry: fastRetry(2)})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsSuccess() {
		t.Error("expected success after a transient transport error")
	}
}
