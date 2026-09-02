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

type staticCall struct {
	position ast.Expr
	callee   *types.Func
}

type functionReachability struct {
	object *types.Func
	calls  []staticCall
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

func runDeviceReachability(pass *analysis.Pass) (any, error) {
	result := make(deviceReachabilityResult)
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
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee, _ := calledObject(pass, call.Fun).(*types.Func)
				if callee == nil {
					return true
				}
				state.calls = append(state.calls, staticCall{position: call.Fun, callee: callee})
				if rule, ok := forbiddenPackageRule(callee); ok {
					state.paths[rule] = chooseReachabilityPath(state.paths[rule], []string{functionName(object), objectNameString(callee)})
				}
				return true
			})
		}
	}

	changed := true
	for changed {
		changed = false
		for _, state := range functions {
			for _, call := range state.calls {
				paths := importedOrLocalPaths(pass, functions, call.callee)
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
		for _, call := range state.calls {
			if call.callee.Pkg() == nil || call.callee.Pkg() == pass.Pkg {
				continue
			}
			if _, forbidden := forbiddenPackageRule(call.callee); forbidden {
				continue
			}
			fact := new(deviceReachabilityFact)
			if !pass.ImportObjectFact(call.callee, fact) {
				continue
			}
			for _, reachable := range fact.Paths {
				result[reachable.Rule] = append(result[reachable.Rule], analysis.Diagnostic{
					Pos: call.position.Pos(), End: call.position.End(),
					Message: fmt.Sprintf("call reaches unavailable device feature: %s", strings.Join(reachable.Path, " -> ")),
				})
			}
		}
	}
	return result, nil
}

func importedOrLocalPaths(pass *analysis.Pass, functions map[*types.Func]*functionReachability, callee *types.Func) map[RuleID][]string {
	if local := functions[callee]; local != nil {
		return local.paths
	}
	if rule, forbidden := forbiddenPackageRule(callee); forbidden {
		return map[RuleID][]string{rule: {objectNameString(callee)}}
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
	return forbiddenImportRule(function.Pkg().Path())
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
