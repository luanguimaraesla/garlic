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
			switch objectName(pass.TypesInfo, call.Fun) {
			case "go.uber.org/zap.New", "go.uber.org/zap.NewProduction",
				"go.uber.org/zap.NewDevelopment", "go.uber.org/zap.NewExample",
				"go.uber.org/zap.NewNop":
				report(pass, call.Pos(), "G2.03", "logger created manually: use Garlic logging helpers")
			}
			return true
		})
	}
	return nil, nil
}
