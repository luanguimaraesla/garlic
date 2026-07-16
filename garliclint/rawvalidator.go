package garliclint

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var RawValidatorAnalyzer = &analysis.Analyzer{Name: "garlicrawvalidator", Doc: "requires validator errors to be converted", Run: runRawValidator}

func runRawValidator(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "validator") {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			ifStmt, ok := node.(*ast.IfStmt)
			if !ok || ifStmt.Init == nil {
				return true
			}
			assign, ok := ifStmt.Init.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) != 1 || !isValidatorCall(assign.Rhs[0]) {
				return true
			}
			errName, ok := assign.Lhs[len(assign.Lhs)-1].(*ast.Ident)
			if !ok {
				return true
			}
			ast.Inspect(ifStmt.Body, func(inner ast.Node) bool {
				ret, ok := inner.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for _, expr := range ret.Results {
					if ident, ok := expr.(*ast.Ident); ok && ident.Name == errName.Name {
						report(pass, ident.Pos(), "G5.01", "raw validator error returned: wrap with validator.ParseValidationErrors(err)")
					}
				}
				return true
			})
			return true
		})
	}
	return nil, nil
}

func isValidatorCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Struct", "StructCtx", "Var", "VarWithValidation":
		return true
	}
	return false
}
