//go:build unit

package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// RequesterMock is a builder-pattern mock of [Requester]. It binds forked
// Requests to a synthetic transport, so the real Request/Response pipeline
// (header merge, body encoding, decode) runs against a canned response. A
// downstream service that depends on [Requester] injects this in its tests
// instead of standing up an httptest server.
type RequesterMock struct {
	status   int
	body     []byte
	header   http.Header
	err      error
	requests *[]*http.Request
}

// NewRequesterMock returns a mock that replies 200 with an empty body.
func NewRequesterMock() RequesterMock {
	return RequesterMock{status: http.StatusOK, header: http.Header{}}
}

// WithStatus sets the response status code.
func (m RequesterMock) WithStatus(code int) RequesterMock {
	m.status = code
	return m
}

// WithBody sets the raw response body.
func (m RequesterMock) WithBody(b []byte) RequesterMock {
	m.body = b
	return m
}

// WithJSON marshals v as the response body and sets a JSON content type.
func (m RequesterMock) WithJSON(v any) RequesterMock {
	b, _ := json.Marshal(v)
	m.body = b
	m.header = cloneHeader(m.header)
	m.header.Set("Content-Type", "application/json")
	return m
}

// WithHeader sets a response header.
func (m RequesterMock) WithHeader(key, value string) RequesterMock {
	m.header = cloneHeader(m.header)
	m.header.Set(key, value)
	return m
}

// WithError makes the synthetic transport fail, simulating a transport-level error.
func (m RequesterMock) WithError(err error) RequesterMock {
	m.err = err
	return m
}

// WithCapture records every outbound *http.Request into the given slice for
// later assertions (see [AssertRequested]).
func (m RequesterMock) WithCapture(into *[]*http.Request) RequesterMock {
	m.requests = into
	return m
}

// R implements [Requester], returning a real Request wired to the canned response.
func (m RequesterMock) R(ctx context.Context) *Request {
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if m.requests != nil {
			*m.requests = append(*m.requests, req)
		}
		if m.err != nil {
			return nil, m.err
		}
		return &http.Response{
			StatusCode: m.status,
			Status:     http.StatusText(m.status),
			Header:     cloneHeader(m.header),
			Body:       io.NopCloser(bytes.NewReader(m.body)),
			Request:    req,
		}, nil
	})

	client, _ := New(&Config{
		BaseURL:    "http://mock.local",
		HTTPClient: &http.Client{Transport: rt},
	})

	return client.R(ctx)
}

var _ Requester = RequesterMock{}

// AssertRequested fails the test unless one of the captured requests matches the
// given method and path.
func AssertRequested(t *testing.T, reqs []*http.Request, method, path string) {
	t.Helper()

	for _, r := range reqs {
		if r.Method == method && r.URL.Path == path {
			return
		}
	}

	t.Errorf("expected a %s request to %q, but it was not among the %d captured request(s)", method, path, len(reqs))
}

func cloneHeader(h http.Header) http.Header {
	if h == nil {
		return http.Header{}
	}
	return h.Clone()
}
