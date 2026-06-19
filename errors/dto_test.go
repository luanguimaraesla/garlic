//go:build unit

package errors

import (
	"net/http"
	"testing"
)

func TestDecodeSafe_knownCode_faithful(t *testing.T) {
	dto := &DTO{
		Name:    "x",
		Error:   "boom",
		Code:    KindNotFoundError.Code,
		Details: map[string]any{"k": "v"},
	}

	e := dto.DecodeSafe(KindSystemError, Hint("ignored"))

	if e.kind != KindNotFoundError {
		t.Errorf("kind = %v, want KindNotFoundError", e.kind.Name)
	}
	if e.message != "boom" {
		t.Errorf("message = %q, want boom", e.message)
	}
	if e.Details["k"] != "v" {
		t.Error("details should be preserved on the faithful path")
	}
	if _, ok := e.Details["hint"]; ok {
		t.Error("fallback opts must not be applied on the faithful path")
	}
}

func TestDecodeSafe_unknownCode_fallbackWithOpts(t *testing.T) {
	dto := &DTO{Error: "weird", Code: "ZZZ999"}
	fallback := KindForStatus(http.StatusServiceUnavailable)

	e := dto.DecodeSafe(fallback, Hint("fell back"))

	if e.kind != fallback {
		t.Errorf("kind = %v, want the fallback kind", e.kind.Name)
	}
	if e.message != "weird" {
		t.Errorf("message = %q, want weird", e.message)
	}
	if e.Details["hint"] != "fell back" {
		t.Error("fallback opts should be applied on the fallback path")
	}
}

func TestDecodeSafe_emptyCode_defaultsToSystemError(t *testing.T) {
	e := (&DTO{Error: "msg only"}).DecodeSafe(nil)
	if e.kind != KindSystemError {
		t.Errorf("nil fallback should default to KindSystemError, got %v", e.kind.Name)
	}
}

func TestDecodeSafe_doesNotPanicOnUnknownCode(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DecodeSafe panicked on unknown code: %v", r)
		}
	}()

	_ = (&DTO{Code: "NOPE"}).DecodeSafe(KindForStatus(http.StatusBadGateway))
}
