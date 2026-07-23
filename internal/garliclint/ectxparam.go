package garliclint

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var EctxParamAnalyzer = &analysis.Analyzer{Name: "garlicectxparam", Doc: "rejects error contexts passed as parameters", Run: runEctxParam}

func runEctxParam(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "errors") {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncType)
			if !ok || fn.Params == nil {
				return true
			}
			for _, field := range fn.Params.List {
				if pointer, ok := pass.TypesInfo.TypeOf(field.Type).(*types.Pointer); ok {
					if named, ok := pointer.Elem().(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == garlicErrorsPath && named.Obj().Name() == "ContextT" {
						report(pass, field.Pos(), "G1.08", "*errors.ContextT passed as a parameter: build errors.Context inline in the function")
					}
				}
			}
			return true
		})
	}
	return nil, nil
}
