package garliclint

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var ManualWriteErrorAnalyzer = &analysis.Analyzer{Name: "garlicmanualwriteerror", Doc: "rejects manual error writes in Garlic handlers", Run: runManualWriteError}

func runManualWriteError(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "rest") {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				if node.Body != nil && isHandler(node.Type, pass.TypesInfo) {
					checkHandlerBody(pass, node.Body)
				}
			case *ast.FuncLit:
				if isHandler(node.Type, pass.TypesInfo) {
					checkHandlerBody(pass, node.Body)
				}
			}
			return true
		})
	}
	return nil, nil
}

// checkHandlerBody reports manual error writes inside a handler body,
// descending into nested non-handler function literals: a closure that
// captures the handler's ResponseWriter is still the handler's code
// path. Handler-shaped literals are skipped; the file walk evaluates
// each of them independently.
func checkHandlerBody(pass *analysis.Pass, body *ast.BlockStmt) {
	for _, stmt := range body.List {
		ast.Inspect(stmt, func(inner ast.Node) bool {
			switch inner := inner.(type) {
			case *ast.FuncLit:
				return !isHandler(inner.Type, pass.TypesInfo)
			case *ast.CallExpr:
				if objectName(pass.TypesInfo, inner.Fun) == "net/http.Error" {
					report(pass, inner.Pos(), "G6.01", "error response written manually via http.Error: return the error and let the route wrapper write it")
				}
				if isWriteErrorCall(inner, pass.TypesInfo) {
					report(pass, inner.Pos(), "G6.06", "rest.WriteError called inside a handler: return the error only")
				}
			}
			return true
		})
	}
}

func isHandler(fn *ast.FuncType, info *types.Info) bool {
	if !returnsError(fn, info) {
		return false
	}
	params := flattenedFieldTypes(fn.Params, info)
	if len(params) != 2 || !isNamedType(params[0], "net/http", "ResponseWriter") {
		return false
	}
	pointer, ok := types.Unalias(params[1]).(*types.Pointer)
	return ok && isNamedType(pointer.Elem(), "net/http", "Request")
}

func isWriteErrorCall(call *ast.CallExpr, info *types.Info) bool {
	return objectName(info, call.Fun) == "github.com/luanguimaraesla/garlic/rest.WriteError"
}
