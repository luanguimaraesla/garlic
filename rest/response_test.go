//go:build unit
// +build unit

package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	generic := errors.KindForStatus(http.StatusInternalServerError)
	if dto.Code != generic.Code {
		t.Errorf("code: want the generic status code %q, got %q", generic.Code, dto.Code)
	}
	if dto.Error != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("error: want the standard status text, got %q", dto.Error)
	}
	if dto.Origin != errors.KindSystemError.Code {
		t.Errorf("origin: want the underlying system code %q, got %q", errors.KindSystemError.Code, dto.Origin)
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
	// System errors are sanitized to the generic kind for their status: the
	// dynamic message becomes the standard status text and the specific code
	// moves to origin, so the client can still quote it to support. The original
	// message and hint never leave the server.
	if dto.Error == "database connection pool exhausted" {
		t.Error("system error message was leaked to the client")
	}
	if dto.Error != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("system error should expose the standard status text, got %q", dto.Error)
	}
	generic := errors.KindForStatus(http.StatusInternalServerError)
	if dto.Code != generic.Code {
		t.Errorf("code: want the generic status code %q, got %q", generic.Code, dto.Code)
	}
	if dto.Origin != errors.KindSystemError.Code {
		t.Errorf("origin: want the underlying system code %q, got %q", errors.KindSystemError.Code, dto.Origin)
	}
	if dto.Details["hint"] == "internal detail" {
		t.Error("the origin's own hint must not cross the wire")
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

func TestWriteError_systemWrappingUserError_staysSanitized(t *testing.T) {
	// A system error that wraps a user-kinded cause must still be sanitized: its
	// own kind (and the HTTP status) is system, so the sensitive message must not
	// leak just because a user error sits in its cause chain.
	userCause := errors.New(errors.KindInvalidRequestError, "public field message")
	err := errors.PropagateAs(errors.KindSystemError, userCause, "secret=xyz connection to 10.0.0.5 failed")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", resp.StatusCode)
	}

	dto := resp.Payload.(*errors.DTO)
	if strings.Contains(dto.Error, "secret=xyz") || strings.Contains(dto.Error, "10.0.0.5") {
		t.Errorf("system error message leaked through a user-kinded cause: %q", dto.Error)
	}
	if dto.Error != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("error: want the standard status text, got %q", dto.Error)
	}
	generic := errors.KindForStatus(http.StatusInternalServerError)
	if dto.Code != generic.Code {
		t.Errorf("code: want the generic status code %q, got %q", generic.Code, dto.Code)
	}
	if dto.Origin != errors.KindSystemError.Code {
		t.Errorf("origin: want the underlying system code %q, got %q", errors.KindSystemError.Code, dto.Origin)
	}
}

func TestWriteError_tertiarySystemKind_codeMovesToOrigin(t *testing.T) {
	// A downstream-registered system kind (tertiary "C" code) parented off a 5xx
	// status. Support needs its specific code, but its description and dynamic
	// message are sensitive and must stay server-side.
	kind := &errors.Kind{
		Name:        "TemporalUnavailable",
		Code:        "C99001",
		Description: "the temporal cluster is unreachable",
		Parent:      errors.KindForStatus(http.StatusServiceUnavailable),
	}
	err := errors.New(kind, "dial tcp 10.0.0.5:7233: connection refused")
	resp := WriteError(err)

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", resp.StatusCode)
	}

	dto := resp.Payload.(*errors.DTO)
	generic := errors.KindForStatus(http.StatusServiceUnavailable)
	if dto.Code != generic.Code {
		t.Errorf("code: want the generic status code %q, got %q", generic.Code, dto.Code)
	}
	if dto.Origin != kind.Code {
		t.Errorf("origin: want the specific kind code %q for support, got %q", kind.Code, dto.Origin)
	}
	if dto.Error != generic.Description {
		t.Errorf("error: want the generic status text, got %q", dto.Error)
	}
	if dto.Name != generic.FQN() {
		t.Errorf("name: want the generic FQN %q, got %q", generic.FQN(), dto.Name)
	}
	if strings.Contains(dto.Name, "TemporalUnavailable") {
		t.Errorf("the specific kind name leaked into the wire body: %q", dto.Name)
	}
	if strings.Contains(dto.Error, "temporal") || strings.Contains(dto.Error, "10.0.0.5") {
		t.Errorf("the specific description or dynamic message leaked: %q", dto.Error)
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
	generic := errors.KindForStatus(http.StatusInternalServerError)
	if dto.Code != generic.Code {
		t.Errorf("code: want the generic status code %q, got %q", generic.Code, dto.Code)
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
