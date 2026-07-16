package garliclint

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var ManualLoggerAnalyzer = &analysis.Analyzer{Name: "garlicmanuallogger", Doc: "rejects manually created Zap loggers", Run: runManualLogger}

func runManualLogger(pass *analysis.Pass) (any, error) {
	if isGarlicPackage(pass, "logging") {
		return nil, nil
	}
	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch sel.Sel.Name {
				case "NewProduction", "NewDevelopment", "NewExample", "NewNop":
					if objectName(pass.TypesInfo, call.Fun) != "" {
						report(pass, call.Pos(), "G2.03", "logger created manually: use Garlic logging helpers")
					}
				}
			}
			return true
		})
	}
	return nil, nil
}
