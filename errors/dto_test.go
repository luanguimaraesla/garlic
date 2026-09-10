//go:build unit

package errors

import (
	"net/http"
	"testing"
)

func TestDTO_DecodeKnownCode(t *testing.T) {
	dto := &DTO{
		Name:    "x",
		Error:   "boom",
		Code:    KindNotFoundError.Code,
		Details: map[string]any{"k": "v"},
	}

	e, ok := dto.Decode()
	if !ok {
		t.Fatal("expected known code to decode")
	}
	if e.kind != KindNotFoundError {
		t.Errorf("kind = %v, want KindNotFoundError", e.kind.Name)
	}
	if e.message != "boom" {
		t.Errorf("message = %q, want boom", e.message)
	}
	if e.Details["k"] != "v" {
		t.Error("details should be preserved")
	}
}

func TestDTO_DecodeUnknownCode(t *testing.T) {
	e, ok := (&DTO{Error: "weird", Code: "ZZZ999"}).Decode()
	if ok {
		t.Fatalf("Decode() ok = true, error = %v", e)
	}
}

func TestDTO_DecodeForKeepsEverythingButTheStatus(t *testing.T) {
	details := map[string]any{"k": "v", "nested": map[string]any{"deep": 1}}
	dto := &DTO{
		Name:    KindNotFoundError.FQN(),
		Error:   "boom",
		Code:    KindNotFoundError.Code,
		Origin:  KindSystemError.Code,
		Details: details,
	}

	e, ok := dto.DecodeFor(599)
	if !ok {
		t.Fatal("expected a registered code to decode")
	}

	if e.StatusCode() != 599 {
		t.Errorf("StatusCode() = %d, want the transport 599", e.StatusCode())
	}
	if !e.Kind().Is(KindNotFoundError) || !e.Kind().Is(KindUserError) {
		t.Errorf("kind = %s, want the peer's kind and its class", e.Kind().FQN())
	}
	if e.Kind().Code != KindNotFoundError.Code || e.Kind().Name != KindNotFoundError.Name {
		t.Errorf("kind identity = %s/%s, want the registered one", e.Kind().Code, e.Kind().Name)
	}
	if e.Kind().Parent != KindNotFoundError.Parent || e.Kind().Description != KindNotFoundError.Description {
		t.Error("the customized copy should keep the registered parent and description")
	}
	if e.message != "boom" || e.Unwrap() != nil {
		t.Errorf("message = %q, cause = %v, want the peer's message and no frame", e.message, e.Unwrap())
	}
	if code, ok := OriginCodeOf(e); !ok || code != KindSystemError.Code {
		t.Errorf("origin = %q, %v, want the peer's", code, ok)
	}
	if e.Details["k"] != "v" {
		t.Errorf("details = %v, want the peer's", e.Details)
	}
	if nested, ok := e.Details["nested"].(map[string]any); !ok || nested["deep"] != 1 {
		t.Errorf("nested details = %v, want them preserved", e.Details["nested"])
	}

	if KindNotFoundError.StatusCode() != http.StatusNotFound {
		t.Errorf("the registered kind now reports %d, want 404", KindNotFoundError.StatusCode())
	}
}

// A status the kind already reports states no intent to pin it, so the
// registered kind is reused rather than copied.
func TestDTO_DecodeForReusesTheKindOnAMatchingStatus(t *testing.T) {
	dto := &DTO{Error: "boom", Code: KindNotFoundError.Code}

	for name, status := range map[string]int{
		"matching":    http.StatusNotFound,
		"unspecified": 0,
		"negative":    -1,
	} {
		t.Run(name, func(t *testing.T) {
			e, ok := dto.DecodeFor(status)
			if !ok {
				t.Fatal("expected a registered code to decode")
			}
			if e.kind != KindNotFoundError {
				t.Errorf("kind = %p, want the registered pointer", e.kind)
			}
			if e.StatusCode() != http.StatusNotFound {
				t.Errorf("StatusCode() = %d, want 404", e.StatusCode())
			}
		})
	}
}

func TestDTO_DecodeForPreservesNilDetails(t *testing.T) {
	e, ok := (&DTO{Error: "boom", Code: KindNotFoundError.Code}).DecodeFor(599)
	if !ok {
		t.Fatal("expected a registered code to decode")
	}
	if e.Details != nil {
		t.Errorf("details = %v, want them left as the DTO had them", e.Details)
	}
}

func TestDTO_DecodeForUnknownCode(t *testing.T) {
	e, ok := (&DTO{Error: "weird", Code: "ZZZ999"}).DecodeFor(599)
	if ok {
		t.Fatalf("DecodeFor() ok = true, error = %v", e)
	}
	if e != nil {
		t.Errorf("error = %v, want nil", e)
	}
}

func TestDTO_MustDecodePanicsOnUnknownCode(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustDecode to panic")
		}
	}()

	_ = (&DTO{Code: "NOPE"}).MustDecode()
}
