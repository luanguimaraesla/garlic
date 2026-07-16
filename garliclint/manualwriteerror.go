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
			fn, ok := node.(*ast.FuncDecl)
			if !ok || !isHandler(fn.Type, pass.TypesInfo) || fn.Body == nil {
				return true
			}
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				call, ok := inner.(*ast.CallExpr)
				if !ok {
					return true
				}
				if objectName(pass.TypesInfo, call.Fun) == "net/http.Error" {
					report(pass, call.Pos(), "G6.01", "error response written manually via http.Error: return the error and let the route wrapper write it")
				}
				if isWriteErrorCall(call, pass.TypesInfo) {
					report(pass, call.Pos(), "G6.06", "rest.WriteError called inside a handler: return the error only")
				}
				return true
			})
			return true
		})
	}
	return nil, nil
}

func isHandler(fn *ast.FuncType, info *types.Info) bool {
	return fn.Params != nil && len(fn.Params.List) == 2 && returnsError(fn, info)
}

func isWriteErrorCall(call *ast.CallExpr, info *types.Info) bool {
	return objectName(info, call.Fun) == "github.com/luanguimaraesla/garlic/rest.WriteError"
}
