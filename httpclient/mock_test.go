//go:build unit

package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestRequesterMock_servesCannedJSON(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}
	mock := NewRequesterMock().WithStatus(http.StatusOK).WithJSON(User{Name: "Alice"})

	var out User
	resp, err := mock.R(context.Background()).SetResult(&out).Get("/users/1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Alice" {
		t.Errorf("decoded name = %q", out.Name)
	}
	if !resp.IsSuccess() {
		t.Error("expected success")
	}
}

func TestRequesterMock_capturesRequest(t *testing.T) {
	var captured []*http.Request
	mock := NewRequesterMock().WithStatus(http.StatusNoContent).WithCapture(&captured)

	if _, err := mock.R(context.Background()).Delete("/users/7"); err != nil {
		t.Fatal(err)
	}
	AssertRequested(t, captured, http.MethodDelete, "/users/7")
}

func TestRequesterMock_transportError(t *testing.T) {
	mock := NewRequesterMock().WithError(fmt.Errorf("network down"))
	if _, err := mock.R(context.Background()).Get("/x"); err == nil {
		t.Fatal("expected an error from the mocked transport failure")
	}
}
