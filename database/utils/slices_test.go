//go:build unit
// +build unit

package utils

import (
	"testing"
)

func TestStringSlice_Scan_string(t *testing.T) {
	var ss StringSlice
	if err := ss.Scan("{a,b,c}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ss) != 3 {
		t.Fatalf("want 3 elements, got %d", len(ss))
	}
	if ss[0] != "a" || ss[1] != "b" || ss[2] != "c" {
		t.Errorf("want [a b c], got %v", ss)
	}
}

func TestStringSlice_Scan_bytes(t *testing.T) {
	var ss StringSlice
	if err := ss.Scan([]byte("{x,y}")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ss) != 2 {
		t.Fatalf("want 2 elements, got %d", len(ss))
	}
	if ss[0] != "x" || ss[1] != "y" {
		t.Errorf("want [x y], got %v", ss)
	}
}

func TestStringSlice_Scan_nil(t *testing.T) {
	ss := StringSlice{"existing"}
	if err := ss.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ss != nil {
		t.Errorf("want nil after scanning nil, got %v", ss)
	}
}

func TestStringSlice_Scan_empty(t *testing.T) {
	var ss StringSlice
	if err := ss.Scan("{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ss) != 0 {
		t.Errorf("want empty slice, got %v", ss)
	}
}

func TestStringSlice_Scan_unsupportedType(t *testing.T) {
	var ss StringSlice
	err := ss.Scan(123)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestStringSlice_Value(t *testing.T) {
	ss := StringSlice{"a", "b", "c"}
	val, err := ss.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := val.(string)
	if !ok {
		t.Fatal("expected string value")
	}
	if got != "{a,b,c}" {
		t.Errorf("want '{a,b,c}', got %q", got)
	}
}

func TestStringSlice_Value_nil(t *testing.T) {
	var ss StringSlice
	val, err := ss.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("want nil, got %v", val)
	}
}

func TestStringSlice_Value_empty(t *testing.T) {
	ss := StringSlice{}
	val, err := ss.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := val.(string)
	if !ok {
		t.Fatal("expected string value")
	}
	if got != "{}" {
		t.Errorf("want '{}', got %q", got)
	}
}

func TestStringSlice_Roundtrip(t *testing.T) {
	original := StringSlice{"hello", "world"}
	val, err := original.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}

	var restored StringSlice
	if err := restored.Scan(val); err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	if len(restored) != len(original) {
		t.Fatalf("want %d elements, got %d", len(original), len(restored))
	}
	for i := range original {
		if restored[i] != original[i] {
			t.Errorf("[%d]: want %q, got %q", i, original[i], restored[i])
		}
	}
}
