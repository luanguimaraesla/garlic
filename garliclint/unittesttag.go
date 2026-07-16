package garliclint

import (
	"go/ast"
	"strings"

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

func hasUnitBuildTag(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.End() > file.Package {
			break
		}
		for _, comment := range group.List {
			if strings.Contains(comment.Text, "go:build") && strings.Contains(comment.Text, "unit") {
				return true
			}
		}
	}
	return false
}
