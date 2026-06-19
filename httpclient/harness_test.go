//go:build unit

package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestServer starts an httptest server and returns a client pointed at it
// with retry disabled (individual tests opt into retry explicitly).
func newTestServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := New(&Config{BaseURL: srv.URL, Retry: RetryConfig{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return client
}

// newClientWithTransport builds a client whose transport is a synthetic
// RoundTripperFunc, for socket-free tests. cfg may be nil.
func newClientWithTransport(t *testing.T, rt RoundTripperFunc, cfg *Config) *Client {
	t.Helper()

	if cfg == nil {
		cfg = &Config{}
	}
	cfg.BaseURL = "http://test.local"
	cfg.Transport = rt

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return client
}

func textResponse(status int, body string, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// fastRetry returns a retry config with tiny waits so retry tests stay quick.
func fastRetry(maxRetries int) RetryConfig {
	return RetryConfig{
		Enabled:    true,
		MaxRetries: maxRetries,
		MinWait:    time.Millisecond,
		MaxWait:    5 * time.Millisecond,
	}
}

// countingRoundTripper records each attempt and the exact request body bytes,
// and delegates the response to respond.
type countingRoundTripper struct {
	mu       sync.Mutex
	attempts int
	bodies   [][]byte
	respond  func(attempt int) (*http.Response, error)
}

func (c *countingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	attempt := c.attempts
	c.attempts++
	c.mu.Unlock()

	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}

	c.mu.Lock()
	c.bodies = append(c.bodies, body)
	c.mu.Unlock()

	return c.respond(attempt)
}

func (c *countingRoundTripper) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.attempts
}
