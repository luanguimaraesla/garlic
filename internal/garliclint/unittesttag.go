package garliclint

import (
	"go/ast"
	"go/build/constraint"

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
// //go:build constraint must be satisfied when exactly the unit tag is
// set. A negated unit constraint and unrelated tokens like unittest are
// rejected, while a constraint such as //go:build !integration still
// passes because it is satisfied under -tags=unit. Malformed expressions
// and legacy // +build lines do not count as tagged.
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
			if expr.Eval(func(tag string) bool { return tag == "unit" }) {
				return true
			}
		}
	}
	return false
}
