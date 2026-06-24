package httpclient

import (
	"context"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy decides whether a request should be retried and how long to wait
// between attempts. It mirrors the CheckRetry + Backoff split popularized by
// hashicorp/go-retryablehttp.
type RetryPolicy interface {
	// CheckRetry reports whether the attempt should be retried. resp is nil on a
	// transport error. attempt is zero-based. Implementations must check
	// ctx.Err() first and return (false, ctxErr) when the context is done.
	CheckRetry(ctx context.Context, method string, resp *http.Response, err error, attempt int) (bool, error)

	// Backoff returns how long to wait before the next attempt. resp may be nil.
	// Honors the Retry-After response header when present.
	Backoff(minWait, maxWait time.Duration, attempt int, resp *http.Response) time.Duration
}

// IsIdempotent reports whether method is retry-safe by default per RFC 9110
// (GET, HEAD, OPTIONS, PUT, DELETE, TRACE).
func IsIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPut, http.MethodDelete, http.MethodTrace:
		return true
	default:
		return false
	}
}

type defaultRetryPolicy struct{}

// DefaultRetryPolicy retries connection errors and the retryable statuses (429,
// 503, and 5xx except 501), honoring Retry-After. Idempotency gating is applied
// by the retry loop, not by the policy.
func DefaultRetryPolicy() RetryPolicy { return defaultRetryPolicy{} }

func (defaultRetryPolicy) CheckRetry(ctx context.Context, _ string, resp *http.Response, err error, _ int) (bool, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, ctxErr
	}

	if err != nil {
		// Transport-level failure (connection reset, timeout): worth retrying.
		return true, nil
	}

	if resp == nil {
		return false, nil
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return true, nil
	}

	if resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented {
		return true, nil
	}

	return false, nil
}

func (defaultRetryPolicy) Backoff(minWait, maxWait time.Duration, attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return clampDuration(d, minWait, maxWait)
		}
	}

	backoff := float64(minWait) * math.Pow(2, float64(attempt))
	if backoff > float64(maxWait) {
		backoff = float64(maxWait)
	}

	// Full +/-50% jitter to avoid synchronized retries (thundering herd). This
	// is backoff timing, not a security-sensitive use of randomness.
	jitter := (rand.Float64() - 0.5) * backoff

	return clampDuration(time.Duration(backoff+jitter), minWait, maxWait)
}

func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}

	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}

	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d, true
		}
		return 0, true
	}

	return 0, false
}

func clampDuration(d, minWait, maxWait time.Duration) time.Duration {
	if d < minWait {
		return minWait
	}
	if d > maxWait {
		return maxWait
	}
	return d
}

// sleepCtx waits for d or until ctx is done. It returns true if the wait
// completed and false if the context was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
