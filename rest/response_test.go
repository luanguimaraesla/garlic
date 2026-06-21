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
	want := errors.KindForStatus(http.StatusInternalServerError)
	if dto.Code != want.Code {
		t.Errorf("code: want %q, got %q", want.Code, dto.Code)
	}
	if dto.Error != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("error: want the standard status text, got %q", dto.Error)
	}
	if dto.Name != "" {
		t.Errorf("name must not cross the wire, got %q", dto.Name)
	}
}

func TestWriteError_systemError_returnsSanitized500(t *testing.T) {
	err := errors.New(errors.KindSystemError, "database connection pool exhausted",
		errors.Hint("internal detail"))
	resp := WriteError(err)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", resp.StatusCode)
	}

	dto, ok := resp.Payload.(*errors.DTO)
	if !ok {
		t.Fatal("payload is not *errors.DTO")
	}
	// System errors are genericized to their HTTP status: the dynamic message is
	// replaced by the standard status text, the name is dropped, and details are
	// stripped entirely.
	want := errors.KindForStatus(http.StatusInternalServerError)
	if dto.Error == "database connection pool exhausted" {
		t.Error("system error message was leaked to the client")
	}
	if dto.Error != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("system error should expose the standard status text, got %q", dto.Error)
	}
	if dto.Code != want.Code {
		t.Errorf("code: want %q, got %q", want.Code, dto.Code)
	}
	if dto.Name != "" {
		t.Errorf("system error name must not cross the wire, got %q", dto.Name)
	}
	if len(dto.Details) != 0 {
		t.Errorf("system error details must be stripped, got %v", dto.Details)
	}
}

func TestWriteError_specificSystemError_preservesStatusAndCode(t *testing.T) {
	unavailable := errors.KindForStatus(http.StatusServiceUnavailable)
	err := errors.New(unavailable, "upstream timeout: 10.0.0.5")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", resp.StatusCode)
	}

	dto := resp.Payload.(*errors.DTO)
	if dto.Code != unavailable.Code {
		t.Errorf("code: want %q, got %q", unavailable.Code, dto.Code)
	}
	if dto.Error == "upstream timeout: 10.0.0.5" {
		t.Error("specific system error message leaked")
	}
	if dto.Error != unavailable.Description {
		t.Errorf("should expose the kind description, got %q", dto.Error)
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

func TestWriteError_invalidRequestError_returns400(t *testing.T) {
	err := errors.New(errors.KindInvalidRequestError, "email is invalid")
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

	dto, ok := resp.Payload.(*errors.DTO)
	if !ok {
		t.Fatal("payload is not *errors.DTO")
	}
	want := errors.KindForStatus(http.StatusInternalServerError)
	if dto.Code != want.Code {
		t.Errorf("code: want %q, got %q", want.Code, dto.Code)
	}
	if dto.Error == "raw stdlib error" {
		t.Error("non-garlic error message was leaked")
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
