package garliclint

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const garlicErrorsPath = "github.com/luanguimaraesla/garlic/errors"

func ruleMsg(id, message string) string { return "[" + id + "] " + message }

func isGarlicPackage(pass *analysis.Pass, name string) bool {
	return pass.Pkg.Path() == "github.com/luanguimaraesla/garlic/"+name
}

func importsGarlicErrors(file *ast.File) bool {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, "\"") == garlicErrorsPath {
			return true
		}
	}
	return false
}

func packageImportsGarlicErrors(pass *analysis.Pass) bool {
	for _, file := range pass.Files {
		if importsGarlicErrors(file) {
			return true
		}
	}
	return false
}

func isNil(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func objectName(info *types.Info, expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		if obj := info.Uses[expr]; obj != nil && obj.Pkg() != nil {
			return obj.Pkg().Path() + "." + obj.Name()
		}
	case *ast.SelectorExpr:
		if obj := info.Uses[expr.Sel]; obj != nil && obj.Pkg() != nil {
			return obj.Pkg().Path() + "." + obj.Name()
		}
	}
	return ""
}

func callObject(info *types.Info, expr ast.Expr) *types.Func {
	var ident *ast.Ident
	switch expr := expr.(type) {
	case *ast.Ident:
		ident = expr
	case *ast.SelectorExpr:
		ident = expr.Sel
	default:
		return nil
	}
	fn, _ := info.Uses[ident].(*types.Func)
	return fn
}

// isGarlicConstructor resolves constructor identity at the call site from the
// used *types.Func, so binding a constructor to a function value
// (p := errors.Propagate; p(err, ...)) is not recognized and such returns are
// still reported by design.
func isGarlicConstructor(call *ast.CallExpr, info *types.Info) bool {
	fn := callObject(info, call.Fun)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != garlicErrorsPath {
		return false
	}

	switch receiverTypeName(fn) {
	case "":
		switch fn.Name() {
		case "Propagate", "PropagateAs", "New", "Raw", "From", "Override", "Mirror", "MirrorOverride":
			return true
		}
	case "ErrorT":
		return fn.Name() == "With"
	case "TemplateT":
		return fn.Name() == "New" || fn.Name() == "Propagate"
	}
	return false
}

func receiverTypeName(fn *types.Func) string {
	signature, ok := fn.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return ""
	}
	typeOfReceiver := signature.Recv().Type()
	if pointer, ok := typeOfReceiver.(*types.Pointer); ok {
		typeOfReceiver = pointer.Elem()
	}
	named, ok := typeOfReceiver.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != garlicErrorsPath {
		return ""
	}
	return named.Obj().Name()
}

func receiverPackagePath(fn *types.Func) string {
	signature, ok := fn.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return ""
	}
	receiver := types.Unalias(signature.Recv().Type())
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	named, ok := receiver.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path()
}

func isErrorType(t types.Type) bool {
	if t == nil {
		return false
	}
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	return types.Implements(t, errorType)
}

func returnsError(fn *ast.FuncType, info *types.Info) bool {
	if fn.Results == nil {
		return false
	}
	for _, field := range fn.Results.List {
		if isErrorType(info.TypeOf(field.Type)) {
			return true
		}
	}
	return false
}

func errorReturnExpressions(ret *ast.ReturnStmt, fn *ast.FuncType, body *ast.BlockStmt, info *types.Info) []ast.Expr {
	if fn.Results == nil {
		return nil
	}
	if len(ret.Results) == 0 {
		return namedErrorResults(fn, body, info)
	}

	resultTypes := declaredResultTypes(fn, info)
	if len(ret.Results) == len(resultTypes) {
		return explicitErrorReturnExpressions(ret.Results, resultTypes)
	}
	if len(ret.Results) == 1 {
		return tupleErrorReturnExpression(ret.Results[0], resultTypes, info)
	}
	return nil
}

func declaredResultTypes(fn *ast.FuncType, info *types.Info) []types.Type {
	return flattenedFieldTypes(fn.Results, info)
}

func flattenedFieldTypes(list *ast.FieldList, info *types.Info) []types.Type {
	if list == nil {
		return nil
	}
	var flattened []types.Type
	for _, field := range list.List {
		fieldType := info.TypeOf(field.Type)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			flattened = append(flattened, fieldType)
		}
	}
	return flattened
}

func isNamedType(t types.Type, path, name string) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == path && obj.Name() == name
}

func explicitErrorReturnExpressions(results []ast.Expr, resultTypes []types.Type) []ast.Expr {
	var expressions []ast.Expr
	for i, expr := range results {
		if isErrorType(resultTypes[i]) {
			expressions = append(expressions, expr)
		}
	}
	return expressions
}

func tupleErrorReturnExpression(result ast.Expr, resultTypes []types.Type, info *types.Info) []ast.Expr {
	tuple, ok := info.TypeOf(result).(*types.Tuple)
	if !ok || tuple.Len() != len(resultTypes) {
		return nil
	}
	for i := range resultTypes {
		if isErrorType(resultTypes[i]) && isErrorType(tuple.At(i).Type()) {
			return []ast.Expr{result}
		}
	}
	return nil
}

func namedErrorResults(fn *ast.FuncType, body *ast.BlockStmt, info *types.Info) []ast.Expr {
	var expressions []ast.Expr
	for _, field := range fn.Results.List {
		if !isErrorType(info.TypeOf(field.Type)) {
			continue
		}
		for _, name := range field.Names {
			if obj := info.Defs[name]; obj != nil && neverAssigned(body, obj, info) {
				continue
			}
			expressions = append(expressions, name)
		}
	}
	return expressions
}

// assignedValues collects every expression assigned to obj within root,
// including inside nested function literals (deferred closures can assign
// named results). opaque marks assignment forms whose value cannot be
// tracked (multi-value assignments, address-taking, range bindings);
// callers must treat opaque objects as holding anything.
func assignedValues(root ast.Node, obj types.Object, info *types.Info) (values []ast.Expr, opaque bool) {
	ast.Inspect(root, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if len(node.Lhs) == len(node.Rhs) {
				for i, lhs := range node.Lhs {
					if identObject(info, lhs) == obj {
						values = append(values, node.Rhs[i])
					}
				}
				return true
			}
			for _, lhs := range node.Lhs {
				if identObject(info, lhs) == obj {
					opaque = true
				}
			}
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if info.Defs[name] != obj {
					continue
				}
				if len(node.Values) == len(node.Names) {
					values = append(values, node.Values[i])
				} else if len(node.Values) > 0 {
					opaque = true
				}
			}
		case *ast.UnaryExpr:
			if node.Op == token.AND && identObject(info, node.X) == obj {
				opaque = true
			}
		case *ast.RangeStmt:
			if identObject(info, node.Key) == obj || identObject(info, node.Value) == obj {
				opaque = true
			}
		}
		return true
	})
	return values, opaque
}

func identObject(info *types.Info, expr ast.Expr) types.Object {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	if obj := info.Defs[ident]; obj != nil {
		return obj
	}
	return info.Uses[ident]
}

func neverAssigned(root ast.Node, obj types.Object, info *types.Info) bool {
	values, opaque := assignedValues(root, obj, info)
	return len(values) == 0 && !opaque
}

func isPropagatedVar(root ast.Node, obj types.Object, info *types.Info) bool {
	values, opaque := assignedValues(root, obj, info)
	if opaque || len(values) == 0 {
		return false
	}
	for _, value := range values {
		call, ok := value.(*ast.CallExpr)
		if !ok || !isGarlicConstructor(call, info) {
			return false
		}
	}
	return true
}

type foreignInterface struct {
	path string
	name string
}

var foreignInterfaces = []foreignInterface{
	{"io", "Reader"}, {"io", "Writer"}, {"io", "Closer"}, {"io", "ReadWriter"},
	{"io", "ReadCloser"}, {"io", "WriteCloser"}, {"io", "ReadWriteCloser"},
	{"io", "ReaderAt"}, {"io", "WriterAt"}, {"io", "Seeker"}, {"io", "StringWriter"},
	{"net/http", "RoundTripper"}, {"net/http", "Handler"}, {"database/sql", "Scanner"},
	{"database/sql/driver", "Valuer"},
}

func isForeignInterfaceMethod(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}

	method, _ := pass.TypesInfo.Defs[fn.Name].(*types.Func)
	receiver := pass.TypesInfo.TypeOf(fn.Recv.List[0].Type)
	if method == nil || receiver == nil || !methodDeclaredOnReceiver(receiver, method) {
		return false
	}

	for _, target := range foreignInterfaces {
		if pkg := importedPackage(pass.Pkg, target.path); pkg != nil {
			if iface := namedInterface(pkg, target.name); iface != nil &&
				isForeignInterfaceMethodMatch(receiver, method, iface, false) {
				return true
			}
			continue
		}

		pkg, err := importer.Default().Import(target.path)
		if err != nil {
			continue
		}
		if iface := namedInterface(pkg, target.name); iface != nil &&
			isForeignInterfaceMethodMatch(receiver, method, iface, true) {
			return true
		}
	}
	return false
}

func importedPackage(root *types.Package, path string) *types.Package {
	seen := make(map[*types.Package]bool)
	var find func(*types.Package) *types.Package
	find = func(pkg *types.Package) *types.Package {
		if pkg == nil || seen[pkg] {
			return nil
		}
		if pkg.Path() == path {
			return pkg
		}
		seen[pkg] = true
		for _, imported := range pkg.Imports() {
			if found := find(imported); found != nil {
				return found
			}
		}
		return nil
	}
	return find(root)
}

func namedInterface(pkg *types.Package, name string) *types.Interface {
	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return nil
	}
	iface, _ := obj.Type().Underlying().(*types.Interface)
	return iface
}

func isForeignInterfaceMethodMatch(receiver types.Type, method *types.Func, iface *types.Interface, canonical bool) bool {
	implements := implementsInterface
	sameSignature := sameMethodSignature
	if canonical {
		implements = implementsCanonicalInterface
		sameSignature = sameCanonicalMethodSignature
	}
	if !implements(receiver, iface) {
		return false
	}
	for i := range iface.NumMethods() {
		interfaceMethod := iface.Method(i)
		if interfaceMethod.Name() == method.Name() && sameSignature(method, interfaceMethod) {
			return true
		}
	}
	return false
}

func methodDeclaredOnReceiver(receiver types.Type, method *types.Func) bool {
	for _, methodSet := range methodSets(receiver) {
		selection := methodSet.Lookup(method.Pkg(), method.Name())
		if selection != nil && selection.Obj() == method {
			return true
		}
	}
	return false
}

func implementsInterface(receiver types.Type, iface *types.Interface) bool {
	for _, candidate := range receiverTypes(receiver) {
		if types.Implements(candidate, iface) {
			return true
		}
	}
	return false
}

func methodSets(receiver types.Type) []*types.MethodSet {
	sets := []*types.MethodSet{types.NewMethodSet(receiver)}
	base := receiverBaseType(receiver)
	if !types.Identical(base, receiver) {
		sets = append(sets, types.NewMethodSet(base))
	}
	return append(sets, types.NewMethodSet(types.NewPointer(base)))
}

func receiverTypes(receiver types.Type) []types.Type {
	base := receiverBaseType(receiver)
	return []types.Type{receiver, base, types.NewPointer(base)}
}

func receiverBaseType(receiver types.Type) types.Type {
	if pointer, ok := receiver.(*types.Pointer); ok {
		return pointer.Elem()
	}
	return receiver
}

func sameMethodSignature(actual, expected *types.Func) bool {
	actualSignature, ok := actual.Type().(*types.Signature)
	if !ok {
		return false
	}
	expectedSignature, ok := expected.Type().(*types.Signature)
	if !ok || actualSignature.Variadic() != expectedSignature.Variadic() {
		return false
	}
	return sameTupleTypes(actualSignature.Params(), expectedSignature.Params()) &&
		sameTupleTypes(actualSignature.Results(), expectedSignature.Results())
}

func sameTupleTypes(first, second *types.Tuple) bool {
	if first.Len() != second.Len() {
		return false
	}
	for i := range first.Len() {
		if !types.Identical(first.At(i).Type(), second.At(i).Type()) {
			return false
		}
	}
	return true
}

func implementsCanonicalInterface(receiver types.Type, iface *types.Interface) bool {
	for _, candidate := range receiverTypes(receiver) {
		methods := types.NewMethodSet(candidate)
		matches := true
		for i := range iface.NumMethods() {
			expected := iface.Method(i)
			if !methodSetHasCanonicalMethod(methods, expected) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func methodSetHasCanonicalMethod(methods *types.MethodSet, expected *types.Func) bool {
	for i := range methods.Len() {
		actual, _ := methods.At(i).Obj().(*types.Func)
		if actual != nil && actual.Name() == expected.Name() && sameCanonicalMethodSignature(actual, expected) {
			return true
		}
	}
	return false
}

func sameCanonicalMethodSignature(actual, expected *types.Func) bool {
	actualSignature, ok := actual.Type().(*types.Signature)
	if !ok {
		return false
	}
	expectedSignature, ok := expected.Type().(*types.Signature)
	if !ok || actualSignature.Variadic() != expectedSignature.Variadic() {
		return false
	}
	return sameCanonicalTupleTypes(actualSignature.Params(), expectedSignature.Params()) &&
		sameCanonicalTupleTypes(actualSignature.Results(), expectedSignature.Results())
}

func sameCanonicalTupleTypes(first, second *types.Tuple) bool {
	if first.Len() != second.Len() {
		return false
	}
	for i := range first.Len() {
		if canonicalTypeString(first.At(i).Type()) != canonicalTypeString(second.At(i).Type()) {
			return false
		}
	}
	return true
}

func canonicalTypeString(t types.Type) string {
	return types.TypeString(t, func(pkg *types.Package) string { return pkg.Path() })
}

func fileName(pass *analysis.Pass, file *ast.File) string { return pass.Fset.File(file.Pos()).Name() }
func isTestFile(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(fileName(pass, file), "_test.go")
}
func report(pass *analysis.Pass, pos token.Pos, id, message string) {
	pass.Reportf(pos, "%s", ruleMsg(id, message))
}
