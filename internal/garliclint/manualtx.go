package garliclint

import (
	"go/ast"
	"strings"

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
			fn := callObject(pass.TypesInfo, call.Fun)
			if fn == nil {
				return true
			}
			switch fn.Name() {
			case "BeginTx", "BeginTxx", "Begin", "Commit", "Rollback":
				if isTransactionPackage(receiverPackagePath(fn)) {
					report(pass, call.Pos(), "G3.01", "manual transaction call: use storer.Transaction(ctx, fn)")
				}
			}
			return true
		})
	}
	return nil, nil
}

var transactionPackageRoots = []string{
	"database/sql",
	"github.com/jmoiron/sqlx",
	"github.com/jackc/pgx",
	"github.com/luanguimaraesla/garlic/database",
}

func isTransactionPackage(path string) bool {
	for _, root := range transactionPackageRoots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func importsDatabase(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Path.Value == "\"github.com/luanguimaraesla/garlic/database\"" {
			return true
		}
	}
	return false
}
