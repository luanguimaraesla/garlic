//go:build unit

package garliclint

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestPropagation(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), PropagationAnalyzer, "propagation", "propagationfallback")
}

func TestDefaultAnalyzers(t *testing.T) {
	analyzers := DefaultAnalyzers()
	if len(analyzers) != 1 || analyzers[0] != PropagationAnalyzer {
		t.Fatalf("DefaultAnalyzers() = %v, want only PropagationAnalyzer", analyzers)
	}
}

func TestAllAnalyzers(t *testing.T) {
	want := []*analysis.Analyzer{
		PropagationAnalyzer,
		StdlibErrorsAnalyzer,
		EctxParamAnalyzer,
		ZapErrorAnalyzer,
		ManualLoggerAnalyzer,
		ManualTxAnalyzer,
		RawValidatorAnalyzer,
		ManualWriteErrorAnalyzer,
		UnitTestTagAnalyzer,
	}
	analyzers := AllAnalyzers()
	if len(analyzers) != len(want) {
		t.Fatalf("AllAnalyzers() returned %d analyzers, want %d", len(analyzers), len(want))
	}
	for i, a := range analyzers {
		if a != want[i] {
			t.Errorf("AllAnalyzers()[%d] = %v, want %v", i, a.Name, want[i].Name)
		}
	}
}

func TestStdlibErrors(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), StdlibErrorsAnalyzer, "stdliberrors")
}

func TestEctxParam(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), EctxParamAnalyzer, "ectxparam")
}

func TestZapError(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ZapErrorAnalyzer, "zaperror")
}

func TestManualLogger(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ManualLoggerAnalyzer, "manuallogger")
}

func TestManualTx(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ManualTxAnalyzer, "manualtx")
}

func TestRawValidator(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), RawValidatorAnalyzer, "rawvalidator")
}

func TestManualWriteError(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ManualWriteErrorAnalyzer, "manualwriteerr")
}

func TestUnitTestTag(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), UnitTestTagAnalyzer, "unittesttag")
}

// TestUnitTestTagUnitBuild runs the analyzer with the unit tag set so
// b_test.go loads and exercises hasUnitBuildTag's compliant branch. The
// GoFiles assertion makes the tag propagation self-verifying: if GOFLAGS
// ever stops reaching the analysistest loader, this test fails loudly
// instead of silently testing nothing.
func TestUnitTestTagUnitBuild(t *testing.T) {
	t.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -tags=unit"))
	results := analysistest.Run(t, analysistest.TestData(), UnitTestTagAnalyzer, "unittesttag")
	for _, result := range results {
		for _, file := range result.Action.Package.GoFiles {
			if strings.HasSuffix(file, "b_test.go") {
				return
			}
		}
	}
	t.Fatal("b_test.go was not loaded; -tags=unit did not reach the analysistest loader")
}

func TestHasUnitBuildTag(t *testing.T) {
	cases := []struct {
		name       string
		constraint string
		want       bool
	}{
		{"unit", "//go:build unit", true},
		{"negated unit", "//go:build !unit", false},
		{"unittest token", "//go:build unittest", false},
		{"unrelated tag", "//go:build integration", false},
		{"unit or integration", "//go:build unit || integration", true},
		{"unit and not race", "//go:build unit && !race", true},
		{"no constraint", "", false},
		{"malformed", "//go:build (unit", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package p\n"
			if tc.constraint != "" {
				src = tc.constraint + "\n\n" + src
			}
			file, err := parser.ParseFile(token.NewFileSet(), "x_test.go", src, parser.ParseComments)
			if err != nil {
				t.Fatalf("failed to parse source: %v", err)
			}
			if got := hasUnitBuildTag(file); got != tc.want {
				t.Errorf("hasUnitBuildTag(%q) = %v, want %v", tc.constraint, got, tc.want)
			}
		})
	}
}
