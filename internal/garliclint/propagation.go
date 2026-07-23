package garliclint

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var PropagationAnalyzer = &analysis.Analyzer{
	Name: "garlicpropagation",
	Doc:  "checks Garlic error propagation conventions",
	Run:  runPropagation,
}

func runPropagation(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "errors") || !packageImportsGarlicErrors(pass) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				checkPropagationFunction(pass, node.Type, node.Body, node)
			case *ast.FuncLit:
				checkPropagationFunction(pass, node.Type, node.Body, nil)
			case *ast.CallExpr:
				checkFmtErrorf(pass, node)
				checkNewWrap(pass, node)
			}
			return true
		})
	}
	return nil, nil
}

func checkPropagationFunction(pass *analysis.Pass, signature *ast.FuncType, body *ast.BlockStmt, decl *ast.FuncDecl) {
	if body == nil || !returnsError(signature, pass.TypesInfo) || (decl != nil && isForeignInterfaceMethod(pass, decl)) {
		return
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			return node != decl
		case *ast.FuncLit:
			return false
		}
		ret, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, expr := range errorReturnExpressions(ret, signature, body, pass.TypesInfo) {
			if isNil(expr) {
				continue
			}
			if call, ok := expr.(*ast.CallExpr); ok && isGarlicConstructor(call, pass.TypesInfo) {
				continue
			}
			if ident, ok := expr.(*ast.Ident); ok && isPropagatedIdent(pass.TypesInfo, body, ident) {
				continue
			}
			if isClassificationCall(expr, pass.TypesInfo) {
				report(pass, expr.Pos(), "G0.02", "classification helper returned bare: wrap with errors.Propagate so the current function joins the reverse trace")
			} else {
				report(pass, expr.Pos(), "G0.01", "error returned without errors.Propagate: bare return breaks the reverse trace")
			}
		}
		return true
	})
	checkLogThenReturn(pass, body)
}

func isPropagatedIdent(info *types.Info, body *ast.BlockStmt, ident *ast.Ident) bool {
	obj := info.Uses[ident]
	if obj == nil {
		obj = info.Defs[ident]
	}
	if obj == nil {
		return false
	}
	return isPropagatedVar(body, obj, info)
}

func isClassificationCall(expr ast.Expr, info *types.Info) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || isGarlicConstructor(call, info) || objectName(info, call.Fun) == "fmt.Errorf" {
		return false
	}
	sig, ok := info.TypeOf(call.Fun).(*types.Signature)
	if !ok || sig.Results().Len() == 0 || !isErrorType(sig.Results().At(sig.Results().Len()-1).Type()) {
		return false
	}
	for _, arg := range call.Args {
		if isErrorType(info.TypeOf(arg)) {
			return true
		}
	}
	return false
}

func checkFmtErrorf(pass *analysis.Pass, call *ast.CallExpr) {
	if objectName(pass.TypesInfo, call.Fun) != "fmt.Errorf" || len(call.Args) < 2 {
		return
	}
	format, ok := call.Args[0].(*ast.BasicLit)
	if !ok || !strings.Contains(format.Value, "%") {
		return
	}
	for _, arg := range call.Args[1:] {
		if isErrorType(pass.TypesInfo.TypeOf(arg)) {
			report(pass, call.Pos(), "G0.04", "fmt.Errorf wraps an error: use errors.Propagate to preserve the reverse trace")
			return
		}
	}
}

func checkNewWrap(pass *analysis.Pass, call *ast.CallExpr) {
	name := objectName(pass.TypesInfo, call.Fun)
	if name != garlicErrorsPath+".New" && name != garlicErrorsPath+".Raw" {
		return
	}
	for _, arg := range call.Args {
		if isErrorType(pass.TypesInfo.TypeOf(arg)) {
			report(pass, call.Pos(), "G0.06", "errors.New used to wrap an existing error: use errors.Propagate to preserve the cause chain")
			return
		}
	}
}

func checkLogThenReturn(pass *analysis.Pass, body *ast.BlockStmt) {
	for _, statement := range body.List {
		block, ok := statement.(*ast.IfStmt)
		if !ok || block.Body == nil {
			continue
		}
		var logged types.Object
		for _, stmt := range block.Body.List {
			ast.Inspect(stmt, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				for _, arg := range call.Args {
					if field, ok := arg.(*ast.CallExpr); ok && (objectName(pass.TypesInfo, field.Fun) == garlicErrorsPath+".Zap" || objectName(pass.TypesInfo, field.Fun) == "go.uber.org/zap.Error") && len(field.Args) == 1 {
						if ident, ok := field.Args[0].(*ast.Ident); ok {
							if obj := pass.TypesInfo.Uses[ident]; obj != nil {
								logged = obj
							}
						}
					}
				}
				return true
			})
			if ret, ok := stmt.(*ast.ReturnStmt); ok && logged != nil {
				for _, expr := range ret.Results {
					ident, ok := expr.(*ast.Ident)
					if !ok || pass.TypesInfo.Uses[ident] != logged || isPropagatedVar(body, logged, pass.TypesInfo) {
						continue
					}
					report(pass, expr.Pos(), "G0.07", "error logged then returned unpropagated: propagate first so the log includes the full reverse trace")
				}
			}
		}
	}
}
