//go:build unit
// +build unit

package database

import (
	"testing"
)

type filterInput struct {
	Name   *string `filter:"name"`
	Age    *int    `filter:"age"`
	NoTag  *string
	Active *bool `filter:"active"`
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }

func TestExtractFilters_allSet(t *testing.T) {
	input := filterInput{
		Name:   strPtr("Alice"),
		Age:    intPtr(30),
		Active: boolPtr(true),
	}

	filters := ExtractFilters(&input)
	if len(filters) != 3 {
		t.Fatalf("want 3 filters, got %d", len(filters))
	}

	found := map[string]bool{}
	for _, f := range filters {
		found[f.key] = true
	}
	for _, key := range []string{"name", "age", "active"} {
		if !found[key] {
			t.Errorf("missing filter key %q", key)
		}
	}
}

func TestExtractFilters_nilsSkipped(t *testing.T) {
	input := filterInput{
		Name: strPtr("Alice"),
		// Age and Active are nil
	}

	filters := ExtractFilters(&input)
	if len(filters) != 1 {
		t.Fatalf("want 1 filter, got %d", len(filters))
	}
	if filters[0].key != "name" {
		t.Errorf("want key 'name', got %q", filters[0].key)
	}
}

func TestExtractFilters_allNil(t *testing.T) {
	input := filterInput{}

	filters := ExtractFilters(&input)
	if len(filters) != 0 {
		t.Errorf("want 0 filters, got %d", len(filters))
	}
}

func TestExtractFilters_nilPointerInput(t *testing.T) {
	var input *filterInput
	filters := ExtractFilters(input)
	if len(filters) != 0 {
		t.Errorf("want 0 filters for nil input, got %d", len(filters))
	}
}

func TestExtractFilters_byValue(t *testing.T) {
	input := filterInput{
		Name: strPtr("Bob"),
	}

	filters := ExtractFilters(input)
	if len(filters) != 1 {
		t.Fatalf("want 1 filter, got %d", len(filters))
	}
}

func TestFilter_Statement(t *testing.T) {
	f := &Filter{key: "name", value: "Alice"}
	got := f.Statement(1)
	want := "name=$1"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestFilter_Value(t *testing.T) {
	f := &Filter{key: "name", value: "Alice"}
	got := f.Value()
	if got != "Alice" {
		t.Errorf("want %q, got %q", "Alice", got)
	}
}

func TestExtractFilters_nonPointerField_panics(t *testing.T) {
	type badInput struct {
		Name string `filter:"name"`
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-pointer filter field")
		}
	}()

	ExtractFilters(badInput{Name: "test"})
}

func TestExtractFilters_nonStruct_panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-struct input")
		}
	}()

	ExtractFilters("not a struct")
}
