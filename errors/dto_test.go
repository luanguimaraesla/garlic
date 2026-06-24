//go:build unit

package errors

import "testing"

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

func TestDTO_MustDecodePanicsOnUnknownCode(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustDecode to panic")
		}
	}()

	_ = (&DTO{Code: "NOPE"}).MustDecode()
}
