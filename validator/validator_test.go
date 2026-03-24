//go:build unit
// +build unit

package validator

import (
	"fmt"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

func TestGlobal_returnsNonNil(t *testing.T) {
	v := Global()
	if v == nil {
		t.Fatal("Global() returned nil")
	}
}

func TestGlobal_returnsSameInstance(t *testing.T) {
	v1 := Global()
	v2 := Global()
	if v1 != v2 {
		t.Error("Global() should return the same instance")
	}
}

func TestValidate_requiredField(t *testing.T) {
	type form struct {
		Name string `json:"name" validate:"required"`
	}

	err := Global().Struct(form{Name: ""})
	if err == nil {
		t.Fatal("expected validation error for empty required field")
	}

	err = Global().Struct(form{Name: "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_emailField(t *testing.T) {
	type form struct {
		Email string `json:"email" validate:"required,email"`
	}

	err := Global().Struct(form{Email: "not-an-email"})
	if err == nil {
		t.Fatal("expected validation error for invalid email")
	}

	err = Global().Struct(form{Email: "alice@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_builtinIsSafePath(t *testing.T) {
	type form struct {
		Path string `json:"path" validate:"is_safe_path"`
	}

	err := Global().Struct(form{Path: "../etc/passwd"})
	if err == nil {
		t.Fatal("expected validation error for path traversal")
	}

	err = Global().Struct(form{Path: "documents/report.pdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_builtinAlphaSpace(t *testing.T) {
	type form struct {
		Name string `json:"name" validate:"alpha_space"`
	}

	err := Global().Struct(form{Name: "Alice Bob"})
	if err != nil {
		t.Fatalf("unexpected error for alpha_space: %v", err)
	}

	err = Global().Struct(form{Name: "Alice123"})
	if err == nil {
		t.Fatal("expected validation error for non-alpha characters")
	}
}

func TestParseValidationErrors_returnsKindValidationError(t *testing.T) {
	type form struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	err := Global().Struct(form{Name: "", Email: "bad"})
	if err == nil {
		t.Fatal("expected validation error")
	}

	parsed := ParseValidationErrors(err)
	if parsed == nil {
		t.Fatal("ParseValidationErrors returned nil")
	}

	if !errors.IsKind(parsed, KindValidationError) {
		t.Error("expected KindValidationError")
	}

	// Check that hint is set
	e, ok := errors.AsKind(parsed, KindValidationError)
	if !ok {
		t.Fatal("could not extract validation error")
	}
	if e.Details["hint"] == nil {
		t.Error("expected hint in details")
	}
	if e.Details["validation"] == nil {
		t.Error("expected validation field details")
	}
}

func TestParseValidationErrors_nil(t *testing.T) {
	result := ParseValidationErrors(nil)
	if result != nil {
		t.Error("ParseValidationErrors(nil) should return nil")
	}
}

func TestParseValidationErrors_nonValidationError(t *testing.T) {
	stdErr := fmt.Errorf("not a validation error")
	result := ParseValidationErrors(stdErr)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !errors.IsKind(result, KindValidationError) {
		t.Error("should still return KindValidationError")
	}
}

func TestParseValidationErrors_usesJSONTagNames(t *testing.T) {
	type form struct {
		FullName string `json:"full_name" validate:"required"`
	}

	err := Global().Struct(form{FullName: ""})
	parsed := ParseValidationErrors(err)

	e, ok := errors.AsKind(parsed, KindValidationError)
	if !ok {
		t.Fatal("could not extract validation error")
	}

	validation, ok := e.Details["validation"].(map[string]string)
	if !ok {
		t.Fatal("validation details not found")
	}

	// Should use JSON tag "full_name", not Go field name "FullName"
	if _, exists := validation["full_name"]; !exists {
		t.Errorf("expected JSON tag 'full_name' in validation details, got keys: %v", validation)
	}
}

func TestExtend_customValidator(t *testing.T) {
	v := New()
	v.Extend(NewValidation("is_even", func(fl Field) bool {
		return fl.Field().Int()%2 == 0
	}))

	type form struct {
		Count int `json:"count" validate:"is_even"`
	}

	if err := v.Struct(form{Count: 4}); err != nil {
		t.Fatalf("unexpected error for even number: %v", err)
	}

	if err := v.Struct(form{Count: 3}); err == nil {
		t.Fatal("expected validation error for odd number")
	}
}
