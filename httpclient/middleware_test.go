//go:build unit

package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestMiddleware_beforeHookOrder(t *testing.T) {
	var order []string
	rt := RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, &Config{
		BeforeRequest: []BeforeRequestHook{
			func(*Client, *Request, *http.Request) error {
				order = append(order, "client")
				return nil
			},
		},
	})

	_, err := c.R(context.Background()).
		OnBeforeRequest(func(*Client, *Request, *http.Request) error {
			order = append(order, "request")
			return nil
		}).
		Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "client" || order[1] != "request" {
		t.Errorf("hook order = %v, want [client request]", order)
	}
}

func TestMiddleware_beforeHookShortCircuits(t *testing.T) {
	called := false
	rt := RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	_, err := c.R(context.Background()).
		OnBeforeRequest(func(*Client, *Request, *http.Request) error {
			return fmt.Errorf("stop")
		}).
		Get("/x")
	if err == nil {
		t.Fatal("expected an error from the before hook")
	}
	if called {
		t.Error("no request should be sent when a before hook fails")
	}
}

func TestMiddleware_afterHookSeesResponse(t *testing.T) {
	rt := RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, "{}", nil), nil
	})
	c := newClientWithTransport(t, rt, nil)

	var status int
	_, err := c.R(context.Background()).
		OnAfterResponse(func(_ *Client, _ *Request, resp *Response) error {
			status = resp.StatusCode
			return nil
		}).
		Get("/x")
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK {
		t.Errorf("after hook saw status %d, want 200", status)
	}
}
