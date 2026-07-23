package garliclint

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var ZapErrorAnalyzer = &analysis.Analyzer{Name: "garliczaperror", Doc: "requires errors.Zap for Garlic errors", Run: runZapError}

func runZapError(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "errors") {
		return nil, nil
	}
	for _, file := range pass.Files {
		if !importsGarlicErrors(file) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || objectName(pass.TypesInfo, call.Fun) != "go.uber.org/zap.Error" || len(call.Args) != 1 {
				return true
			}
			if isErrorType(pass.TypesInfo.TypeOf(call.Args[0])) {
				report(pass, call.Pos(), "G2.02", "zap.Error(err) used in Garlic code: use errors.Zap(err) to preserve reverse trace and context")
			}
			return true
		})
	}
	return nil, nil
}
