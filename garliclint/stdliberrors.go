package garliclint

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

var StdlibErrorsAnalyzer = &analysis.Analyzer{Name: "garlicstdliberr", Doc: "rejects stdlib errors in Garlic-aware files", Run: runStdlibErrors}

func runStdlibErrors(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "errors") {
		return nil, nil
	}
	for _, file := range pass.Files {
		if !importsGarlicErrors(file) {
			continue
		}
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, "\"") == "errors" {
				report(pass, imp.Pos(), "G0.05", "stdlib errors imported in Garlic code: use github.com/luanguimaraesla/garlic/errors instead")
			}
		}
	}
	return nil, nil
}
