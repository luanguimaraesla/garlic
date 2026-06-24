//go:build unit

package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"
)

// trackingReadCloser records whether Close was called.
type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

// failingTokenSource always fails, simulating an expired or unreadable token.
func failingTokenSource() TokenSource {
	return TokenSourceFunc(func() (string, error) {
		return "", fmt.Errorf("token unavailable")
	})
}

// When auth injection fails after the body is installed, a stream body wrapping
// an io.ReadCloser (e.g. an open file) must be closed so its descriptor is not
// leaked, and the transport must not be reached.
func TestExecute_streamBodyClosedWhenAuthFails(t *testing.T) {
	transportCalled := false
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		transportCalled = true
		return textResponse(http.StatusOK, "", nil), nil
	}), &Config{TokenSource: failingTokenSource()})

	body := &trackingReadCloser{Reader: bytes.NewReader([]byte("payload"))}

	_, err := c.R(context.Background()).SetBody(body).Post("/upload")
	if err == nil {
		t.Fatal("expected an auth error")
	}
	if !body.closed {
		t.Error("stream body was not closed on the abort path: file descriptor leak")
	}
	if transportCalled {
		t.Error("transport should not be reached when auth injection fails")
	}
}

// A multipart body spawns a writer goroutine blocked on the pipe. When auth
// injection fails before the request is sent, closing the body must unblock and
// drain that goroutine; otherwise each failed send leaks one.
func TestExecute_multipartGoroutineNotLeakedWhenAuthFails(t *testing.T) {
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, "", nil), nil
	}), &Config{TokenSource: failingTokenSource()})

	before := runtime.NumGoroutine()

	const sends = 50
	for i := 0; i < sends; i++ {
		_, err := c.R(context.Background()).
			SetFileReader("file", "f.bin", bytes.NewReader([]byte("data"))).
			Post("/upload")
		if err == nil {
			t.Fatal("expected an auth error")
		}
	}

	// Goroutines unwind asynchronously; poll back toward the baseline.
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before+5 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if leaked := runtime.NumGoroutine() - before; leaked > 5 {
		t.Errorf("multipart writer goroutines leaked across %d failed sends: delta %d", sends, leaked)
	}
}

// A buffer-backed io.Reader passed to SetBody must stay replayable so an
// idempotent request still retries and resends the body byte-identically.
func TestSetBody_bufferBackedReaderIsRetried(t *testing.T) {
	crt := &countingRoundTripper{
		respond: func(attempt int) (*http.Response, error) {
			if attempt == 0 {
				return textResponse(http.StatusServiceUnavailable, "", nil), nil
			}
			return textResponse(http.StatusOK, "ok", nil), nil
		},
	}
	c := newClientWithTransport(t, RoundTripperFunc(crt.RoundTrip), &Config{Retry: fastRetry(3)})

	resp, err := c.R(context.Background()).
		SetBody(bytes.NewReader([]byte("payload"))).
		Put("/x")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	defer func() { _ = resp.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if crt.count() != 2 {
		t.Errorf("attempts = %d, want 2 (buffer-backed reader should be retried)", crt.count())
	}
	for i, b := range crt.bodies {
		if string(b) != "payload" {
			t.Errorf("attempt %d body = %q, want %q", i, b, "payload")
		}
	}
}
