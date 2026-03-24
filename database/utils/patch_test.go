//go:build unit
// +build unit

package utils

import (
	"sort"
	"strings"
	"testing"
)

type patchResource struct {
	Name  *string `db:"name"`
	Email *string `db:"email"`
	Age   *int    `db:"age"`
	NoTag *string
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestNamedResourceBindings_allSet(t *testing.T) {
	res := &patchResource{
		Name:  strPtr("Alice"),
		Email: strPtr("alice@example.com"),
		Age:   intPtr(30),
	}

	bindings := NamedResourceBindings(res)
	sort.Strings(bindings)

	if len(bindings) != 3 {
		t.Fatalf("want 3 bindings, got %d", len(bindings))
	}

	expected := []string{"age = :age", "email = :email", "name = :name"}
	for i, want := range expected {
		if bindings[i] != want {
			t.Errorf("binding[%d]: want %q, got %q", i, want, bindings[i])
		}
	}
}

func TestNamedResourceBindings_nilsSkipped(t *testing.T) {
	res := &patchResource{
		Name: strPtr("Alice"),
		// Email and Age are nil
	}

	bindings := NamedResourceBindings(res)
	if len(bindings) != 1 {
		t.Fatalf("want 1 binding, got %d", len(bindings))
	}
	if bindings[0] != "name = :name" {
		t.Errorf("want 'name = :name', got %q", bindings[0])
	}
}

func TestNamedResourceBindings_allNil(t *testing.T) {
	res := &patchResource{}

	bindings := NamedResourceBindings(res)
	if len(bindings) != 0 {
		t.Errorf("want 0 bindings, got %d", len(bindings))
	}
}

func TestJoinedPatchResourceBindings(t *testing.T) {
	res := &patchResource{
		Name:  strPtr("Alice"),
		Email: strPtr("alice@example.com"),
	}

	joined := JoinedPatchResourceBindings(res)
	// Order may vary, check both parts exist
	if !strings.Contains(joined, "name = :name") {
		t.Errorf("missing 'name = :name' in %q", joined)
	}
	if !strings.Contains(joined, "email = :email") {
		t.Errorf("missing 'email = :email' in %q", joined)
	}
	if !strings.Contains(joined, ", ") {
		t.Errorf("expected comma separator in %q", joined)
	}
}

func TestJoinedPatchResourceBindings_single(t *testing.T) {
	res := &patchResource{
		Name: strPtr("Alice"),
	}

	joined := JoinedPatchResourceBindings(res)
	if joined != "name = :name" {
		t.Errorf("want 'name = :name', got %q", joined)
	}
}

func TestResourceIter(t *testing.T) {
	res := &patchResource{
		Name:  strPtr("Alice"),
		Email: nil,
		Age:   intPtr(25),
	}

	collected := map[string]any{}
	for k, v := range ResourceIter(res) {
		collected[k] = v
	}

	if len(collected) != 2 {
		t.Fatalf("want 2 entries, got %d", len(collected))
	}
	if collected["name"] != "Alice" {
		t.Errorf("name: want Alice, got %v", collected["name"])
	}
	if collected["age"] != 25 {
		t.Errorf("age: want 25, got %v", collected["age"])
	}
}

func TestResourceIter_empty(t *testing.T) {
	res := &patchResource{}

	count := 0
	for range ResourceIter(res) {
		count++
	}

	if count != 0 {
		t.Errorf("want 0 iterations for all-nil struct, got %d", count)
	}
}

func TestResourceIter_nonPointerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-struct pointer")
		}
	}()

	s := "not a struct"
	for range ResourceIter(&s) {
		// should not reach here
	}
}

func TestNamed(t *testing.T) {
	type args struct {
		Name string `db:"name"`
	}

	query, params := Named("SELECT * FROM users WHERE name = :name", args{Name: "Alice"})
	if !strings.Contains(query, "$1") {
		t.Errorf("expected positional param $1 in %q", query)
	}
	if len(params) != 1 {
		t.Fatalf("want 1 param, got %d", len(params))
	}
	if params[0] != "Alice" {
		t.Errorf("want Alice, got %v", params[0])
	}
}
