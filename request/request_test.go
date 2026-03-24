//go:build unit
// +build unit

package request

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
	"github.com/luanguimaraesla/garlic/tracing"
)

// helper that creates a GET request with a logger in context
func newGetRequest(target string) *http.Request {
	r := httptest.NewRequest("GET", target, nil)
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	return r.WithContext(ctx)
}

// helper that creates a GET request with chi URL params
func newGetRequestWithParam(requestPath string, params map[string]string) *http.Request {
	r := httptest.NewRequest("GET", requestPath, nil)
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)

	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// --- ParseResourceUUID ---

func TestParseResourceUUID_valid(t *testing.T) {
	id := uuid.New()
	r := newGetRequestWithParam("/items/"+id.String(), map[string]string{"id": id.String()})

	got, err := ParseResourceUUID(r, "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Errorf("want %s, got %s", id, got)
	}
}

func TestParseResourceUUID_invalid(t *testing.T) {
	r := newGetRequestWithParam("/items/not-a-uuid", map[string]string{"id": "not-a-uuid"})

	_, err := ParseResourceUUID(r, "id")
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if !errors.IsKind(err, InvalidRequestError) {
		t.Errorf("expected InvalidRequestError kind, got %v", err)
	}
}

// --- ParseResourceInt ---

func TestParseResourceInt_valid(t *testing.T) {
	r := newGetRequestWithParam("/items/42", map[string]string{"page": "42"})

	got, err := ParseResourceInt(r, "page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("want 42, got %d", got)
	}
}

func TestParseResourceInt_invalid(t *testing.T) {
	r := newGetRequestWithParam("/items/abc", map[string]string{"page": "abc"})

	_, err := ParseResourceInt(r, "page")
	if err == nil {
		t.Fatal("expected error for invalid int, got nil")
	}
	if !errors.IsKind(err, InvalidRequestError) {
		t.Errorf("expected InvalidRequestError kind, got %v", err)
	}
}

// --- ParseResourceString ---

func TestParseResourceString_valid(t *testing.T) {
	r := newGetRequestWithParam("/items/my-item", map[string]string{"slug": "my-item"})

	got, err := ParseResourceString(r, "slug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "my-item" {
		t.Errorf("want my-item, got %s", got)
	}
}

func TestParseResourceString_empty(t *testing.T) {
	r := newGetRequestWithParam("/items/", map[string]string{"slug": ""})

	_, err := ParseResourceString(r, "slug")
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

func TestParseResourceString_encoded(t *testing.T) {
	r := newGetRequestWithParam("/items/hello%20world", map[string]string{"slug": "hello%20world"})

	got, err := ParseResourceString(r, "slug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("want 'hello world', got %q", got)
	}
}

// --- ParseParamPagination ---

func TestParseParamPagination_valid(t *testing.T) {
	r := newGetRequest("/items?limit=10&start=5")

	limit, start := ParseParamPagination(r)
	if limit != 10 {
		t.Errorf("limit: want 10, got %d", limit)
	}
	if start != 5 {
		t.Errorf("start: want 5, got %d", start)
	}
}

func TestParseParamPagination_defaults(t *testing.T) {
	r := newGetRequest("/items")

	limit, start := ParseParamPagination(r)
	if limit != 0 {
		t.Errorf("limit: want 0, got %d", limit)
	}
	if start != 0 {
		t.Errorf("start: want 0, got %d", start)
	}
}

func TestParseParamPagination_invalidValues(t *testing.T) {
	r := newGetRequest("/items?limit=abc&start=xyz")

	limit, start := ParseParamPagination(r)
	if limit != 0 {
		t.Errorf("limit: want 0, got %d", limit)
	}
	if start != 0 {
		t.Errorf("start: want 0, got %d", start)
	}
}

// --- ParseParamUUID ---

func TestParseParamUUID_valid(t *testing.T) {
	id := uuid.New()
	r := newGetRequest("/items?user_id=" + id.String())

	got, err := ParseParamUUID(r, "user_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Errorf("want %s, got %s", id, got)
	}
}

func TestParseParamUUID_missing(t *testing.T) {
	r := newGetRequest("/items")

	_, err := ParseParamUUID(r, "user_id")
	if err == nil {
		t.Fatal("expected error for missing param, got nil")
	}
}

func TestParseParamUUID_invalid(t *testing.T) {
	r := newGetRequest("/items?user_id=not-valid")

	_, err := ParseParamUUID(r, "user_id")
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
}

// --- ParseOptionalParamUUID ---

func TestParseOptionalParamUUID_present(t *testing.T) {
	id := uuid.New()
	r := newGetRequest("/items?user_id=" + id.String())

	got, err := ParseOptionalParamUUID(r, "user_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Errorf("want %s, got %s", id, got)
	}
}

func TestParseOptionalParamUUID_absent(t *testing.T) {
	r := newGetRequest("/items")

	got, err := ParseOptionalParamUUID(r, "user_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != uuid.Nil {
		t.Errorf("want uuid.Nil, got %s", got)
	}
}

func TestParseOptionalParamUUID_invalid(t *testing.T) {
	r := newGetRequest("/items?user_id=bad")

	_, err := ParseOptionalParamUUID(r, "user_id")
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
}

// --- ParseParamString ---

func TestParseParamString_present(t *testing.T) {
	r := newGetRequest("/items?name=alice")

	got, err := ParseParamString(r, "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alice" {
		t.Errorf("want alice, got %s", got)
	}
}

func TestParseParamString_missing(t *testing.T) {
	r := newGetRequest("/items")

	_, err := ParseParamString(r, "name")
	if err == nil {
		t.Fatal("expected error for missing param, got nil")
	}
}

// --- ParseOptionalParamBool ---

func TestParseOptionalParamBool_true(t *testing.T) {
	r := newGetRequest("/items?active=true")

	got, err := ParseOptionalParamBool(r, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("want true, got false")
	}
}

func TestParseOptionalParamBool_false(t *testing.T) {
	r := newGetRequest("/items?active=false")

	got, err := ParseOptionalParamBool(r, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("want false, got true")
	}
}

func TestParseOptionalParamBool_absent(t *testing.T) {
	r := newGetRequest("/items")

	got, err := ParseOptionalParamBool(r, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("want false (default), got true")
	}
}

func TestParseOptionalParamBool_invalid(t *testing.T) {
	r := newGetRequest("/items?active=maybe")

	_, err := ParseOptionalParamBool(r, "active")
	if err == nil {
		t.Fatal("expected error for invalid bool, got nil")
	}
}

// --- DecodeRequestBody ---

type testForm struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email"`
}

func TestDecodeRequestBody_valid(t *testing.T) {
	body, _ := json.Marshal(testForm{Name: "Alice", Email: "alice@example.com"})
	r := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	r = r.WithContext(ctx)

	var form testForm
	err := DecodeRequestBody(r, &form)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form.Name != "Alice" {
		t.Errorf("want Alice, got %s", form.Name)
	}
}

func TestDecodeRequestBody_invalidJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte("{invalid")))
	r.Header.Set("Content-Type", "application/json")
	r.ContentLength = 8
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	r = r.WithContext(ctx)

	var form testForm
	err := DecodeRequestBody(r, &form)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !errors.IsKind(err, InvalidRequestError) {
		t.Errorf("expected InvalidRequestError, got %v", err)
	}
}

func TestDecodeRequestBody_validationError(t *testing.T) {
	body, _ := json.Marshal(testForm{Name: "", Email: "alice@example.com"})
	r := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.ContentLength = int64(len(body))
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	r = r.WithContext(ctx)

	var form testForm
	err := DecodeRequestBody(r, &form)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestDecodeRequestBody_emptyBody(t *testing.T) {
	r := httptest.NewRequest("POST", "/users", nil)
	r.ContentLength = 0
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	r = r.WithContext(ctx)

	// Empty form with required Name field should fail validation
	form := &testForm{}
	err := DecodeRequestBody(r, form)
	if err == nil {
		t.Fatal("expected validation error for empty body with required field, got nil")
	}
}

// --- Tracing helpers ---

func TestSetAndGetRequestId(t *testing.T) {
	id := uuid.New()
	r := newGetRequest("/test")
	r = SetRequestId(r, id)

	got, err := GetRequestId(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Errorf("want %s, got %s", id, got)
	}
}

func TestGetRequestId_missing(t *testing.T) {
	r := newGetRequest("/test")
	_, err := GetRequestId(r)
	if err == nil {
		t.Fatal("expected error for missing request id, got nil")
	}
}

func TestSetAndGetSessionId(t *testing.T) {
	r := newGetRequest("/test")
	r = SetSessionId(r, "session-abc")

	got, err := GetSessionId(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "session-abc" {
		t.Errorf("want session-abc, got %s", got)
	}
}

func TestGetSessionId_missing(t *testing.T) {
	r := newGetRequest("/test")
	_, err := GetSessionId(r)
	if err == nil {
		t.Fatal("expected error for missing session id, got nil")
	}
}

// --- SetLogger / GetLogger ---

func TestSetAndGetLogger(t *testing.T) {
	r := newGetRequest("/test")
	logger := zap.NewNop()

	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	r = r.WithContext(ctx)

	got := GetLogger(r)
	if got == nil {
		t.Fatal("expected non-nil logger")
	}
}

// --- RouteContainsPattern ---

func TestRouteContainsPattern_match(t *testing.T) {
	r := httptest.NewRequest("GET", "/orgs/123/items", nil)
	rctx := chi.NewRouteContext()
	rctx.RoutePatterns = []string{"/orgs/{organization_id}/items"}
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	if !RouteContainsPattern(r, `\{organization_id\}`) {
		t.Error("expected pattern to match")
	}
}

func TestRouteContainsPattern_noMatch(t *testing.T) {
	r := httptest.NewRequest("GET", "/items", nil)
	rctx := chi.NewRouteContext()
	rctx.RoutePatterns = []string{"/items"}
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)

	if RouteContainsPattern(r, `\{organization_id\}`) {
		t.Error("expected pattern not to match")
	}
}

// --- ValidateForm ---

func TestValidateForm_valid(t *testing.T) {
	form := &testForm{Name: "Alice", Email: "alice@example.com"}
	if err := ValidateForm(form); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateForm_invalid(t *testing.T) {
	form := &testForm{Name: ""}
	err := ValidateForm(form)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

// --- ParseForm ---

type userModel struct {
	Name string
}

type userForm struct {
	Name string `json:"name" validate:"required"`
}

func (f *userForm) ToModel() (userModel, error) {
	return userModel{Name: f.Name}, nil
}

func TestParseForm_valid(t *testing.T) {
	body, _ := json.Marshal(userForm{Name: "Bob"})
	r := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.ContentLength = int64(len(body))
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	r = r.WithContext(ctx)

	model, err := ParseForm[userModel](r, &userForm{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Name != "Bob" {
		t.Errorf("want Bob, got %s", model.Name)
	}
}

func TestParseForm_invalidBody(t *testing.T) {
	r := httptest.NewRequest("POST", "/users", bytes.NewReader([]byte("bad json")))
	r.ContentLength = 8
	logger := zap.NewNop()
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	r = r.WithContext(ctx)

	_, err := ParseForm[userModel](r, &userForm{})
	if err == nil {
		t.Fatal("expected error for invalid body, got nil")
	}
}

// --- request tracing round-trip ---

func TestTracingRoundTrip(t *testing.T) {
	r := newGetRequest("/test")
	id := uuid.New()
	r = SetRequestId(r, id)
	r = SetSessionId(r, "sess-42")

	gotReqID, _ := tracing.GetRequestIdFromContext(r.Context())
	if gotReqID != id {
		t.Errorf("request id: want %s, got %s", id, gotReqID)
	}

	gotSess, _ := tracing.GetSessionIdFromContext(r.Context())
	if gotSess != "sess-42" {
		t.Errorf("session id: want sess-42, got %s", gotSess)
	}
}
