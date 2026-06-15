//go:build unit
// +build unit

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGoSum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.sum")
	// Mix of:
	//   * an h1: line and its matching /go.mod line (must dedupe to one pin)
	//   * a second module with only the /go.mod variant
	//   * a blank line and a malformed line (must skip)
	//   * a duplicated module@version pair (must dedupe)
	content := `github.com/example/foo v1.2.3 h1:abc=
github.com/example/foo v1.2.3/go.mod h1:def=
github.com/example/bar v0.4.0/go.mod h1:ghi=

malformed-line
github.com/example/foo v1.2.3 h1:abc=
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write go.sum: %v", err)
	}

	pins, err := parseGoSum(path)
	if err != nil {
		t.Fatalf("parseGoSum: %v", err)
	}

	want := []modulePin{
		{Module: "github.com/example/foo", Version: "v1.2.3"},
		{Module: "github.com/example/bar", Version: "v0.4.0"},
	}
	if len(pins) != len(want) {
		t.Fatalf("got %d pins, want %d: %+v", len(pins), len(want), pins)
	}
	for i, p := range pins {
		if p != want[i] {
			t.Errorf("pin[%d] = %+v, want %+v", i, p, want[i])
		}
	}
}

func TestParseGoSumMissingFile(t *testing.T) {
	_, err := parseGoSum(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing go.sum, got nil")
	}
}

func TestEscapePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase only", "github.com/foo/bar", "github.com/foo/bar"},
		{"mixed case", "github.com/Azure/azure-sdk-for-go", "github.com/!azure/azure-sdk-for-go"},
		{"all uppercase segment", "github.com/AAA", "github.com/!a!a!a"},
		{"empty", "", ""},
		{"version with capital", "V1.2.3", "!v1.2.3"},
		{"digits and punct untouched", "v1.2.3-rc1", "v1.2.3-rc1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapePath(tc.in)
			if got != tc.want {
				t.Errorf("escapePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLoadAllowlistMissingFile(t *testing.T) {
	a, warnings, err := loadAllowlist(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil allowlist for missing file")
	}
	if len(a.Exclude) != 0 {
		t.Errorf("expected empty Exclude, got %+v", a.Exclude)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for missing file, got %v", warnings)
	}
}

func TestLoadAllowlistWarnings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.yaml")
	// Three entries: missing expires, unparseable expires, past expires.
	// All three should emit a warning.
	content := `exclude:
  - module: github.com/example/missing
    version: v1.0.0
    reason: "no expiry"
    approved_by: "@x"
    added: 2026-01-01
  - module: github.com/example/garbage
    version: v1.0.0
    reason: "garbage expiry"
    approved_by: "@x"
    added: 2026-01-01
    expires: "not-a-date"
  - module: github.com/example/expired
    version: v1.0.0
    reason: "past expiry"
    approved_by: "@x"
    added: 2025-01-01
    expires: 2025-02-01
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}
	_, warnings, err := loadAllowlist(path)
	if err != nil {
		t.Fatalf("loadAllowlist: %v", err)
	}
	if len(warnings) != 3 {
		t.Fatalf("want 3 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestMatchesFailsClosed(t *testing.T) {
	a := &allowlist{
		Exclude: []allowlistEntry{
			{
				Module:  "github.com/example/active",
				Version: "v1.0.0",
				Expires: "2099-01-01",
			},
			{
				Module:  "github.com/example/expired",
				Version: "v1.0.0",
				Expires: "2020-01-01",
			},
			{
				Module:  "github.com/example/garbage",
				Version: "v1.0.0",
				Expires: "not-a-date",
			},
			{
				Module:  "github.com/example/missing",
				Version: "v1.0.0",
				// no Expires
			},
			{
				Module:  "github.com/example/wildcard",
				Version: "",
				Expires: "2099-01-01",
			},
		},
	}
	now := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		pin  modulePin
		want bool
	}{
		{"active matches", modulePin{"github.com/example/active", "v1.0.0"}, true},
		{"version mismatch", modulePin{"github.com/example/active", "v2.0.0"}, false},
		{"expired fails closed", modulePin{"github.com/example/expired", "v1.0.0"}, false},
		{"unparseable expires fails closed", modulePin{"github.com/example/garbage", "v1.0.0"}, false},
		{"missing expires fails closed", modulePin{"github.com/example/missing", "v1.0.0"}, false},
		{"wildcard with valid expiry matches any version", modulePin{"github.com/example/wildcard", "v9.9.9"}, true},
		{"unknown module", modulePin{"github.com/example/none", "v1.0.0"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := a.matches(tc.pin, now)
			if got != tc.want {
				t.Errorf("matches(%+v) = %v, want %v", tc.pin, got, tc.want)
			}
		})
	}
}
