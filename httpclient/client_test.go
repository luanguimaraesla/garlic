//go:build unit

package httpclient

import (
	"context"
	"net/http"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

func TestNew_nilConfigUsesDefaults(t *testing.T) {
	c, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}
	if c.config.BaseURL != "http://localhost" {
		t.Errorf("BaseURL = %q", c.config.BaseURL)
	}
}

func TestNew_badBaseURL(t *testing.T) {
	_, err := New(&Config{BaseURL: "://bad"})
	if err == nil {
		t.Fatal("expected an error for a bad base URL")
	}
	if !errors.IsKind(err, errors.KindSystemError) {
		t.Errorf("want a system error, got %v", err)
	}
}

func TestNew_injectedHTTPClientHonored(t *testing.T) {
	called := false
	rt := RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return textResponse(http.StatusOK, "{}", nil), nil
	})

	c, err := New(&Config{BaseURL: "http://x.local", HTTPClient: &http.Client{Transport: rt}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.R(context.Background()).Get("/"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("the injected HTTP client was not used")
	}
}

func TestNew_pooledTransportIsolatedAndUntimed(t *testing.T) {
	c1, _ := New(&Config{BaseURL: "http://x.local"})
	c2, _ := New(&Config{BaseURL: "http://y.local"})

	hc1, ok1 := c1.doer.(*http.Client)
	hc2, ok2 := c2.doer.(*http.Client)
	if !ok1 || !ok2 {
		t.Fatal("expected an *http.Client doer")
	}
	if _, ok := hc1.Transport.(*http.Transport); !ok {
		t.Error("expected a pooled *http.Transport")
	}
	if hc1.Transport == hc2.Transport {
		t.Error("each client should get its own isolated pooled transport")
	}
	if hc1.Timeout != 0 {
		t.Error("http.Client.Timeout must remain unset so context deadlines take effect")
	}
}

func TestClient_decodesJSONResult(t *testing.T) {
	c, _ := New(&Config{
		BaseURL: "http://x.local",
		HTTPClient: &http.Client{Transport: RoundTripperFunc(func(*http.Request) (*http.Response, error) {
			return textResponse(http.StatusOK, `{"name":"Alice"}`, nil), nil
		})},
	})

	var out struct {
		Name string `json:"name"`
	}
	resp, err := c.R(context.Background()).SetResult(&out).Get("/users/1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Alice" {
		t.Errorf("decoded name = %q", out.Name)
	}
	if !resp.IsSuccess() {
		t.Error("expected IsSuccess")
	}
}
