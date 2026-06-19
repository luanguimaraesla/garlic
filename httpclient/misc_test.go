//go:build unit

package httpclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

func TestRequest_settersAndResponseGetters(t *testing.T) {
	var (
		captured *http.Request
		body     []byte
	)
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}
		return textResponse(http.StatusOK, `{"ok":true}`, nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	resp, err := c.R(context.Background()).
		SetHeaders(map[string]string{"X-A": "1"}).
		SetQueryParam("q", "v").
		SetQueryParams(map[string]string{"r": "w"}).
		SetContentType("application/json").
		SetBodyJSON(map[string]int{"n": 1}).
		SetContext(context.Background()).
		Patch("/x")
	if err != nil {
		t.Fatal(err)
	}

	if captured.Header.Get("X-A") != "1" {
		t.Error("SetHeaders not applied")
	}
	if captured.URL.Query().Get("q") != "v" || captured.URL.Query().Get("r") != "w" {
		t.Errorf("query params not applied: %s", captured.URL.RawQuery)
	}
	if captured.Header.Get("Content-Type") != "application/json" {
		t.Error("SetContentType not applied")
	}
	if string(body) != `{"n":1}` {
		t.Errorf("body = %s", body)
	}

	if resp.Status() == "" {
		t.Error("Status empty")
	}
	if resp.Header() == nil {
		t.Error("Header nil")
	}
	if resp.RawResponse() == nil {
		t.Error("RawResponse nil")
	}
	if resp.IsError() {
		t.Error("200 should not be IsError")
	}
	if !strings.Contains(resp.String(), "ok") {
		t.Errorf("String = %q", resp.String())
	}
}

func TestRequest_headAndOptions(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	if _, err := c.R(context.Background()).Head("/x"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.R(context.Background()).Options("/x"); err != nil {
		t.Fatal(err)
	}
}

func TestRequest_basicAuth(t *testing.T) {
	var (
		user, pass string
		ok         bool
	)
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		user, pass, ok = req.BasicAuth()
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	if _, err := c.R(context.Background()).SetBasicAuth("u", "p").Get("/x"); err != nil {
		t.Fatal(err)
	}
	if !ok || user != "u" || pass != "p" {
		t.Errorf("basic auth = %q/%q (%v)", user, pass, ok)
	}
}

func TestRequest_setBodyVariants(t *testing.T) {
	var body []byte
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(req.Body)
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	cases := map[string]func(*Request) *Request{
		"raw-string":  func(r *Request) *Request { return r.SetBody("raw-string") },
		"reader-body": func(r *Request) *Request { return r.SetBodyReader(strings.NewReader("reader-body")) },
		"byte-body":   func(r *Request) *Request { return r.SetBody([]byte("byte-body")) },
	}
	for want, set := range cases {
		if _, err := set(c.R(context.Background())).Post("/x"); err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Errorf("body = %q, want %q", body, want)
		}
	}
}

func TestRequest_invalidBodyFailsAtSend(t *testing.T) {
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, "{}", nil), nil
	}), nil)

	if _, err := c.R(context.Background()).SetBodyJSON(make(chan int)).Post("/x"); err == nil {
		t.Fatal("expected an error for an unmarshalable body")
	}
}

func TestBuildURL(t *testing.T) {
	u, err := buildURL("http://h/base", "/users", map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if u != "http://h/base/users?a=b" {
		t.Errorf("joined URL = %q", u)
	}

	u, err = buildURL("", "http://absolute/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if u != "http://absolute/x" {
		t.Errorf("absolute URL = %q", u)
	}
}

func TestResponseError_bodyAccessor(t *testing.T) {
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusInternalServerError, "boom-body", nil), nil
	}), &Config{Retry: RetryConfig{}})

	_, err := c.R(context.Background()).Get("/x")
	var re *ResponseError
	if !errors.As(err, &re) {
		t.Fatal("expected a ResponseError")
	}
	if re.Body() != "boom-body" {
		t.Errorf("Body() = %q", re.Body())
	}
}

func TestRequesterMock_withBodyAndHeader(t *testing.T) {
	mock := NewRequesterMock().WithStatus(http.StatusOK).WithBody([]byte("hello")).WithHeader("X-Custom", "yes")

	resp, err := mock.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if resp.String() != "hello" {
		t.Errorf("body = %q", resp.String())
	}
	if resp.Header().Get("X-Custom") != "yes" {
		t.Errorf("header = %q", resp.Header().Get("X-Custom"))
	}
}
