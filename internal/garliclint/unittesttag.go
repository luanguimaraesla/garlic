package garliclint

import (
	"go/ast"
	"go/build/constraint"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

var UnitTestTagAnalyzer = &analysis.Analyzer{Name: "garlicunittesttag", Doc: "requires unit build tags in test files", Run: runUnitTestTag}

func runUnitTestTag(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if !isTestFile(pass, file) || hasUnitBuildTag(file) {
			continue
		}
		report(pass, file.Package, "G8.01", "test file missing //go:build unit")
	}
	return nil, nil
}

// hasUnitBuildTag reports whether the file counts as unit-tagged: its
// //go:build constraint must be satisfied under -tags=unit and must NOT
// be satisfied without it. Platform tokens (GOOS, GOARCH, go1.N release
// tags) are treated as implicitly satisfiable on both sides, so forms
// like //go:build unit && linux or unit && go1.24 count as tagged while
// //go:build linux alone does not. A negated unit constraint and
// unrelated tokens like unittest are rejected, and a unit-independent
// constraint such as //go:build !integration is satisfied with or
// without -tags=unit, so it correctly does not count as tagged (the file
// builds in a plain `go test` run without the tag). Malformed
// expressions and legacy // +build lines do not count as tagged.
func hasUnitBuildTag(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.End() > file.Package {
			break
		}
		for _, comment := range group.List {
			if !constraint.IsGoBuild(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil {
				continue
			}
			withUnit := expr.Eval(func(tag string) bool { return tag == "unit" || knownPlatformTag(tag) })
			withoutUnit := expr.Eval(knownPlatformTag)
			if withUnit && !withoutUnit {
				return true
			}
		}
	}
	return false
}

// knownGOOS and knownGOARCH mirror the GOOS/GOARCH values printed by
// `go tool dist list`; go/build does not export its known lists. These
// tokens (and go1.N release tags, matched below) are satisfied
// implicitly by some toolchain, unlike -tags values such as unit.
var knownGOOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "illumos": true, "ios": true, "js": true,
	"linux": true, "netbsd": true, "openbsd": true, "plan9": true,
	"solaris": true, "wasip1": true, "windows": true,
}

var knownGOARCH = map[string]bool{
	"386": true, "amd64": true, "arm": true, "arm64": true,
	"loong64": true, "mips": true, "mips64": true, "mips64le": true,
	"mipsle": true, "ppc64": true, "ppc64le": true, "riscv64": true,
	"s390x": true, "wasm": true,
}

var goReleaseTag = regexp.MustCompile(`^go1\.[0-9]+$`)

func knownPlatformTag(tag string) bool {
	return knownGOOS[tag] || knownGOARCH[tag] || goReleaseTag.MatchString(tag)
}
