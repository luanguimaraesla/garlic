package garliclint

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var ManualTxAnalyzer = &analysis.Analyzer{Name: "garlicmanualtx", Doc: "rejects manual database transaction handling", Run: runManualTx}

func runManualTx(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "database") {
		return nil, nil
	}
	for _, file := range pass.Files {
		if !importsDatabase(file) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "BeginTx", "BeginTxx", "Begin", "Commit", "Rollback":
				report(pass, call.Pos(), "G3.01", "manual transaction call: use storer.Transaction(ctx, fn)")
			}
			return true
		})
	}
	return nil, nil
}

func importsDatabase(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Path.Value == "\"github.com/luanguimaraesla/garlic/database\"" {
			return true
		}
	}
	return false
}
