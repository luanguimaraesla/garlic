package garliclint

import (
	"go/ast"
	"go/types"
	"strings"

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
			if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) != 1 || !isValidatorCall(pass.TypesInfo, assign.Rhs[0]) {
				return true
			}
			errName, ok := assign.Lhs[len(assign.Lhs)-1].(*ast.Ident)
			if !ok {
				return true
			}
			errObj := pass.TypesInfo.Defs[errName]
			if errObj == nil {
				errObj = pass.TypesInfo.Uses[errName]
			}
			if errObj == nil {
				return true
			}
			ast.Inspect(ifStmt.Body, func(inner ast.Node) bool {
				ret, ok := inner.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for _, expr := range ret.Results {
					if ident, ok := expr.(*ast.Ident); ok && pass.TypesInfo.Uses[ident] == errObj {
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

func isValidatorCall(info *types.Info, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	fn := callObject(info, call.Fun)
	if fn == nil {
		return false
	}
	switch fn.Name() {
	case "Struct", "StructCtx", "Var", "VarWithValidation":
		return isValidatorPackage(receiverPackagePath(fn))
	}
	return false
}

func isValidatorPackage(path string) bool {
	return path == "github.com/luanguimaraesla/garlic/validator" ||
		path == "github.com/go-playground/validator" ||
		strings.HasPrefix(path, "github.com/go-playground/validator/")
}
