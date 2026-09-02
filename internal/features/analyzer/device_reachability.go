package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type deviceReachabilityFact struct {
	Paths []deviceReachabilityPath
}

type deviceReachabilityPath struct {
	Rule RuleID
	Path []string
}

func (*deviceReachabilityFact) AFact() {}

type deviceReachabilityResult map[RuleID][]analysis.Diagnostic

type functionReachability struct {
	object *types.Func
	calls  []*types.Func
	paths  map[RuleID][]string
}

func newDeviceReachabilityProvider() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:       "devicereachability",
		Doc:        "compute shortest known static paths to forbidden device features",
		ResultType: reflect.TypeOf(deviceReachabilityResult{}),
		FactTypes:  []analysis.Fact{new(deviceReachabilityFact)},
		Run:        runDeviceReachability,
	}
}

func deviceReachabilityRule(name string, provider *analysis.Analyzer, rule RuleID) *analysis.Analyzer {
	return &analysis.Analyzer{Name: name, Doc: "report reachable forbidden device features", Requires: []*analysis.Analyzer{provider}, Run: func(pass *analysis.Pass) (any, error) {
		for _, diagnostic := range pass.ResultOf[provider].(deviceReachabilityResult)[rule] {
			pass.Report(diagnostic)
		}
		return nil, nil
	}}
}

func addDeviceReachability(implementation, provider *analysis.Analyzer, rule RuleID) {
	implementation.Requires = append(implementation.Requires, provider)
	run := implementation.Run
	implementation.Run = func(pass *analysis.Pass) (any, error) {
		result, err := run(pass)
		if err != nil {
			return result, err
		}
		for _, diagnostic := range pass.ResultOf[provider].(deviceReachabilityResult)[rule] {
			pass.Report(diagnostic)
		}
		return result, nil
	}
}

func runDeviceReachability(pass *analysis.Pass) (any, error) {
	result := make(deviceReachabilityResult)
	bindings := localFunctionBindings(pass)
	functions := make(map[*types.Func]*functionReachability)
	for _, file := range pass.Files {
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if rule, ok := forbiddenImportRule(path); ok {
				result[rule] = append(result[rule], analysis.Diagnostic{Pos: spec.Path.Pos(), Message: forbiddenImportMessage(rule)})
			}
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			object, _ := pass.TypesInfo.Defs[function.Name].(*types.Func)
			if object == nil {
				continue
			}
			state := &functionReachability{object: object, paths: make(map[RuleID][]string)}
			functions[object] = state
			calls, features := staticDeviceOperations(pass, function.Body, bindings)
			state.calls = calls
			// Channel types in a signature are part of the function contract.
			ast.Inspect(function.Type, func(node ast.Node) bool { collectDeviceSyntax(pass, node, features); return true })
			for rule, cause := range features {
				state.paths[rule] = []string{functionName(object), cause}
			}
		}
	}

	changed := true
	for changed {
		changed = false
		for _, state := range functions {
			for _, callee := range state.calls {
				paths := importedOrLocalPaths(pass, functions, callee)
				for rule, path := range paths {
					candidate := append([]string{functionName(state.object)}, path...)
					selected := chooseReachabilityPath(state.paths[rule], candidate)
					if strings.Join(selected, "\x00") != strings.Join(state.paths[rule], "\x00") {
						state.paths[rule] = selected
						changed = true
					}
				}
			}
		}
	}

	for _, state := range functions {
		if state.object.Exported() && len(state.paths) != 0 {
			pass.ExportObjectFact(state.object, &deviceReachabilityFact{Paths: sortedReachabilityPaths(state.paths)})
		}
	}
	reportDeviceInitialization(pass, functions, result, bindings)
	// Reporting covers the entire file, including package variable initializers
	// and function literals that have no declared function object of their own.
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := resolvedDeviceCallee(pass, call, bindings)
			if callee == nil || callee.Pkg() == nil || callee.Pkg() == pass.Pkg {
				return true
			}
			if _, forbidden := forbiddenPackageRule(callee); forbidden {
				return true
			}
			if deviceSymbolBoundary(callee.Pkg().Path()) {
				return true
			}
			fact := new(deviceReachabilityFact)
			if !pass.ImportObjectFact(callee, fact) {
				return true
			}
			for _, reachable := range fact.Paths {
				result[reachable.Rule] = append(result[reachable.Rule], analysis.Diagnostic{
					Pos: call.Fun.Pos(), End: call.Fun.End(),
					Message: fmt.Sprintf("call reaches unavailable device feature: %s", strings.Join(reachable.Path, " -> ")),
				})
			}
			return true
		})
	}
	return result, nil
}

// staticDeviceOperations follows syntactic calls, including immediately invoked function
// literals and immutable local aliases of functions and closures. Creating a
// closure does not execute its body; other values require dynamic analysis.
func staticDeviceOperations(pass *analysis.Pass, node ast.Node, bindings map[*types.Var]ast.Expr) ([]*types.Func, map[RuleID]string) {
	var calls []*types.Func
	features := make(map[RuleID]string)
	visited := make(map[*ast.FuncLit]bool)
	var inspect func(ast.Node) bool
	inspect = func(node ast.Node) bool {
		if _, literal := node.(*ast.FuncLit); literal {
			return false
		}
		collectDeviceSyntax(pass, node, features)
		if call, ok := node.(*ast.CallExpr); ok {
			if callee := resolvedDeviceCallee(pass, call, bindings); callee != nil {
				calls = append(calls, callee)
			} else if literal, ok := resolvedDeviceFunctionValue(pass, call.Fun, bindings).(*ast.FuncLit); ok && !visited[literal] {
				visited[literal] = true
				ast.Inspect(literal.Body, inspect)
			}
		}
		return true
	}
	ast.Inspect(node, inspect)
	return calls, features
}

func collectDeviceSyntax(pass *analysis.Pass, node ast.Node, features map[RuleID]string) {
	add := func(rule RuleID, cause string) {
		if old := features[rule]; old == "" || cause < old {
			features[rule] = cause
		}
	}
	switch node := node.(type) {
	case *ast.GoStmt:
		add("device-goroutine", "go statement")
	case *ast.SelectStmt:
		add("device-select", "select statement")
	case *ast.CallExpr:
		if objectName(pass, node.Fun, "builtin", "recover") {
			add("device-recover", "recover()")
		}
	}
	// Reuse the direct channel rule, including typed ranges and generic
	// arguments, without publishing dependency-local diagnostics.
	local := *pass
	local.Report = func(diagnostic analysis.Diagnostic) { add("device-channel", diagnostic.Message) }
	reportChannel(&local, node)
}

func reportDeviceInitialization(pass *analysis.Pass, functions map[*types.Func]*functionReachability, result deviceReachabilityResult, bindings map[*types.Var]ast.Expr) {
	paths := make(map[RuleID][]string)
	name := pass.Pkg.Path() + " initialization"
	add := func(rule RuleID, path []string) {
		paths[rule] = chooseReachabilityPath(paths[rule], append([]string{name}, path...))
	}
	for _, state := range functions {
		if state.object.Name() == "init" && state.object.Signature().Recv() == nil {
			for rule, path := range state.paths {
				add(rule, path)
			}
		}
	}
	for _, initializer := range pass.TypesInfo.InitOrder {
		calls, features := staticDeviceOperations(pass, initializer.Rhs, bindings)
		for rule, cause := range features {
			add(rule, []string{cause})
		}
		for _, callee := range calls {
			for rule, path := range importedOrLocalPaths(pass, functions, callee) {
				add(rule, path)
			}
		}
	}
	for _, file := range pass.Files {
		for _, spec := range file.Imports {
			object := pass.TypesInfo.PkgNameOf(spec)
			if object == nil {
				continue
			}
			imported := object.Imported()
			if _, forbidden := forbiddenImportRule(imported.Path()); forbidden {
				continue
			}
			if deviceSymbolBoundary(imported.Path()) {
				continue
			}
			fact := new(deviceReachabilityFact)
			if !pass.ImportPackageFact(imported, fact) {
				continue
			}
			for _, reachable := range fact.Paths {
				add(reachable.Rule, reachable.Path)
				result[reachable.Rule] = append(result[reachable.Rule], analysis.Diagnostic{
					Pos: spec.Path.Pos(), End: spec.Path.End(),
					Message: fmt.Sprintf("import initializes unavailable device feature: %s", strings.Join(reachable.Path, " -> ")),
				})
			}
		}
	}
	if len(paths) != 0 {
		pass.ExportPackageFact(&deviceReachabilityFact{Paths: sortedReachabilityPaths(paths)})
	}
}

func importedOrLocalPaths(pass *analysis.Pass, functions map[*types.Func]*functionReachability, callee *types.Func) map[RuleID][]string {
	if rule, forbidden := forbiddenPackageRule(callee); forbidden {
		return map[RuleID][]string{rule: {objectNameString(callee)}}
	}
	if callee.Pkg() != nil && deviceSymbolBoundary(callee.Pkg().Path()) {
		return nil
	}
	if local := functions[callee]; local != nil {
		return local.paths
	}
	if callee.Pkg() == nil || callee.Pkg() == pass.Pkg {
		return nil
	}
	fact := new(deviceReachabilityFact)
	if !pass.ImportObjectFact(callee, fact) {
		return nil
	}
	paths := make(map[RuleID][]string, len(fact.Paths))
	for _, reachable := range fact.Paths {
		paths[reachable.Rule] = reachable.Path
	}
	return paths
}

func forbiddenImportRule(path string) (RuleID, bool) {
	switch path {
	case "fmt":
		return "device-fmt", true
	case "encoding/json":
		return "device-encoding-json", true
	default:
		return "", false
	}
}

func forbiddenPackageRule(function *types.Func) (RuleID, bool) {
	if function.Pkg() == nil {
		return "", false
	}
	if rule, forbidden := forbiddenImportRule(function.Pkg().Path()); forbidden {
		return rule, true
	}
	return deviceSymbolRule(function)
}

// Audited runtime symbols are terminal contracts. Their host Go implementation
// does not describe the device implementation of an allowed SDK subset.
func deviceSymbolBoundary(path string) bool {
	return path == "time" || path == "reflect" || path == "runtime"
}

func forbiddenImportMessage(rule RuleID) string {
	if rule == "device-fmt" {
		return "fmt is unavailable on device; use strconv and bounded writers"
	}
	return "encoding/json is unavailable on device; use playdate/json"
}

func chooseReachabilityPath(current, candidate []string) []string {
	if len(candidate) == 0 {
		return current
	}
	if len(current) == 0 || len(candidate) < len(current) || len(candidate) == len(current) && strings.Join(candidate, "\x00") < strings.Join(current, "\x00") {
		return append([]string(nil), candidate...)
	}
	return current
}

func sortedReachabilityPaths(paths map[RuleID][]string) []deviceReachabilityPath {
	rules := make([]RuleID, 0, len(paths))
	for rule := range paths {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i] < rules[j] })
	result := make([]deviceReachabilityPath, 0, len(rules))
	for _, rule := range rules {
		result = append(result, deviceReachabilityPath{Rule: rule, Path: append([]string(nil), paths[rule]...)})
	}
	return result
}

func functionName(function *types.Func) string {
	return objectNameString(function)
}

func objectNameString(object types.Object) string {
	return types.ObjectString(object, func(pkg *types.Package) string { return pkg.Path() })
}
