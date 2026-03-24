//go:build unit
// +build unit

package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

func TestMust_setsContentTypeHeader(t *testing.T) {
	w := httptest.NewRecorder()
	WriteMessage(http.StatusOK, "ok").Must(w)

	got := w.Header().Get("Content-Type")
	if got != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", got)
	}
}

func TestMust_setsStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	WriteMessage(http.StatusCreated, "created").Must(w)

	if w.Code != http.StatusCreated {
		t.Errorf("status code: want %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestMust_encodesPayloadAsJSON(t *testing.T) {
	type user struct {
		Name string `json:"name"`
	}

	w := httptest.NewRecorder()
	WriteResponse(http.StatusOK, user{Name: "Alice"}).Must(w)

	var got user
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if got.Name != "Alice" {
		t.Errorf("want Alice, got %s", got.Name)
	}
}

func TestWriteMessage_wrapsInPayloadMessage(t *testing.T) {
	w := httptest.NewRecorder()
	WriteMessage(http.StatusOK, "hello").Must(w)

	var got PayloadMessage
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if got.Message != "hello" {
		t.Errorf("want hello, got %s", got.Message)
	}
}

func TestWriteError_nil_returnsUnknownError(t *testing.T) {
	resp := WriteError(nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", resp.StatusCode)
	}

	dto, ok := resp.Payload.(*errors.DTO)
	if !ok {
		t.Fatal("payload is not *errors.DTO")
	}
	if dto.Error != "unknown error" {
		t.Errorf("error message: want 'unknown error', got %q", dto.Error)
	}
}

func TestWriteError_systemError_returnsSanitized500(t *testing.T) {
	err := errors.New(errors.KindSystemError, "database connection pool exhausted")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", resp.StatusCode)
	}

	dto, ok := resp.Payload.(*errors.DTO)
	if !ok {
		t.Fatal("payload is not *errors.DTO")
	}
	// System errors must be sanitized, never leaking the real message
	if dto.Error == "database connection pool exhausted" {
		t.Error("system error message was leaked to the client")
	}
}

func TestWriteError_userError_returnsDetailedResponse(t *testing.T) {
	err := errors.New(errors.KindNotFoundError, "user not found",
		errors.Hint("check the user ID"),
	)
	resp := WriteError(err)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", resp.StatusCode)
	}

	dto, ok := resp.Payload.(*errors.DTO)
	if !ok {
		t.Fatal("payload is not *errors.DTO")
	}
	if dto.Error != "user not found" {
		t.Errorf("error: want 'user not found', got %q", dto.Error)
	}
	if dto.Details["hint"] != "check the user ID" {
		t.Errorf("hint: want 'check the user ID', got %v", dto.Details["hint"])
	}
}

func TestWriteError_validationError_returns400(t *testing.T) {
	err := errors.New(errors.KindValidationError, "email is invalid")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", resp.StatusCode)
	}
}

func TestWriteError_nonGarlicError_returnsSanitized500(t *testing.T) {
	err := fmt.Errorf("raw stdlib error")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", resp.StatusCode)
	}
}

func TestWriteError_propagatedUserError_preservesKind(t *testing.T) {
	cause := errors.New(errors.KindNotFoundError, "record missing")
	wrapped := errors.Propagate(cause, "service failed")
	resp := WriteError(wrapped)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", resp.StatusCode)
	}
}

func TestWriteError_authError_returns401(t *testing.T) {
	err := errors.New(errors.KindAuthError, "invalid credentials")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: want 401, got %d", resp.StatusCode)
	}
}

func TestWriteError_forbiddenError_returns403(t *testing.T) {
	err := errors.New(errors.KindForbiddenError, "access denied")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status: want 403, got %d", resp.StatusCode)
	}
}
