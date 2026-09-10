package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type performanceFindings map[RuleID][]analysis.Diagnostic

type hotFunction struct {
	name  string
	pos   token.Pos
	body  *ast.BlockStmt
	calls []*types.Func
	risks []hotRisk
}

type hotRisk struct {
	rule    RuleID
	pos     token.Pos
	end     token.Pos
	message string
}

type hotRoot struct {
	function *hotFunction
	audio    bool
}

func performanceRuleRegistrations() []Registration {
	provider := &analysis.Analyzer{Name: "hotpaths", Doc: "classify structural risks reachable from frame and audio callbacks", ResultType: reflect.TypeOf(performanceFindings{}), Run: runPerformanceRules}
	ids := []RuleID{"performance-frame-allocation", "performance-frame-resource-load", "performance-frame-unbounded-work", "performance-audio-callback-risk"}
	result := make([]Registration, 0, len(ids))
	for _, id := range ids {
		result = append(result, Registration{RuleID: id, Analyzer: &analysis.Analyzer{Name: strings.ReplaceAll(string(id), "-", "_"), Doc: "report a structural hot-path risk requiring measurement", Requires: []*analysis.Analyzer{provider}, Run: func(pass *analysis.Pass) (any, error) {
			for _, diagnostic := range pass.ResultOf[provider].(performanceFindings)[id] {
				pass.Report(diagnostic)
			}
			return nil, nil
		}}})
	}
	return result
}

func runPerformanceRules(pass *analysis.Pass) (any, error) {
	result := make(performanceFindings)
	functions := make(map[*types.Func]*hotFunction)
	var roots []hotRoot
	for _, file := range pass.Files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			object, _ := pass.TypesInfo.Defs[function.Name].(*types.Func)
			if object == nil {
				continue
			}
			state := &hotFunction{name: hotFunctionName(object), pos: function.Name.Pos(), body: function.Body}
			functions[object] = state
			if isGameUpdate(object) {
				roots = append(roots, hotRoot{function: state})
			}
		}
	}
	for _, state := range functions {
		state.calls, state.risks = inspectHotBody(pass, state.body, functions)
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, audio := callbackRegistration(pass, call)
			if name == "" {
				return true
			}
			for _, argument := range call.Args {
				if identifier, ok := argument.(*ast.Ident); ok {
					if object, ok := pass.TypesInfo.Uses[identifier].(*types.Func); ok && functions[object] != nil {
						roots = append(roots, hotRoot{function: functions[object], audio: audio})
					}
				}
				literal, ok := argument.(*ast.FuncLit)
				if !ok {
					continue
				}
				state := &hotFunction{name: name + " callback", pos: literal.Type.Func, body: literal.Body}
				state.calls, state.risks = inspectHotBody(pass, literal.Body, functions)
				roots = append(roots, hotRoot{function: state, audio: audio})
			}
			return true
		})
	}
	for _, root := range roots {
		walkHotPaths(root, functions, result)
	}
	for id := range result {
		sort.Slice(result[id], func(i, j int) bool { return result[id][i].Pos < result[id][j].Pos })
	}
	return result, PassContext(pass).Err()
}

func inspectHotBody(pass *analysis.Pass, body *ast.BlockStmt, functions map[*types.Func]*hotFunction) ([]*types.Func, []hotRisk) {
	var calls []*types.Func
	var risks []hotRisk
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		switch item := node.(type) {
		case *ast.ForStmt:
			if !boundedForLoop(pass, item) {
				risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.For, end: item.Body.Lbrace, message: "loop has no statically visible termination condition"})
			}
			risks = append(risks, loopGrowthRisks(pass, item.Body)...)
		case *ast.RangeStmt:
			if ranged := pass.TypesInfo.TypeOf(item.X); ranged != nil {
				switch ranged.Underlying().(type) {
				case *types.Slice, *types.Map, *types.Chan:
					risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.For, end: item.Body.Lbrace, message: "iteration count has no statically visible bound"})
				}
			}
			risks = append(risks, loopGrowthRisks(pass, item.Body)...)
		case *ast.SendStmt:
			risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Arrow, end: item.End(), message: "channel operation may block on a frame path"})
		case *ast.SelectStmt:
			risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Select, end: item.Body.Lbrace, message: "select may block or perform work without a derivable bound"})
		case *ast.UnaryExpr:
			if item.Op == token.ARROW {
				risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.OpPos, end: item.End(), message: "channel receive may block on a frame path"})
			} else if item.Op == token.AND {
				if _, ok := item.X.(*ast.CompositeLit); ok {
					risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.OpPos, end: item.End(), message: "addressable value construction is reachable on a frame path"})
				}
			}
		case *ast.CallExpr:
			callee := staticHotCallee(pass, item)
			if callee != nil {
				if _, ok := functions[callee]; ok {
					calls = append(calls, callee)
					if functions[callee].body == body {
						risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Pos(), end: item.End(), message: "recursive call is reachable on a frame path"})
					}
					if isGameUpdate(callee) {
						risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Pos(), end: item.End(), message: "game update re-entry is reachable from a callback"})
					}
				}
				path, name := packageAndName(callee)
				switch {
				case path == "runtime" && name == "GC":
					risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Pos(), end: item.End(), message: "explicit garbage collection is reachable on a frame path"})
				case path == "reflect":
					risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.Pos(), end: item.End(), message: "reflection is reachable on a frame path and may allocate"})
				case isResourceOrIOCall(path, name):
					risks = append(risks, hotRisk{rule: "performance-frame-resource-load", pos: item.Pos(), end: item.End(), message: "resource loading or I/O is reachable on a frame path"})
				case isBlockingCall(path, name):
					risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Pos(), end: item.End(), message: "potentially blocking work is reachable on a frame path"})
				}
			}
			if identifier, ok := item.Fun.(*ast.Ident); ok && identifier.Name == "make" {
				risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.Pos(), end: item.End(), message: "make allocation is reachable on a frame path"})
			}
			if identifier, ok := item.Fun.(*ast.Ident); ok && identifier.Name == "new" {
				risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.Pos(), end: item.End(), message: "new allocation is reachable on a frame path"})
			}
			if identifier, ok := item.Fun.(*ast.Ident); ok && identifier.Name == "append" {
				risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.Pos(), end: item.End(), message: "append with unproven spare capacity is reachable on a frame path"})
			}
		case *ast.CompositeLit:
			typeOf := pass.TypesInfo.TypeOf(item)
			if typeOf != nil {
				if _, ok := typeOf.Underlying().(*types.Slice); ok {
					risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.Pos(), end: item.End(), message: "slice construction is reachable on a frame path"})
				}
				if _, ok := typeOf.Underlying().(*types.Map); ok {
					risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.Pos(), end: item.End(), message: "map construction is reachable on a frame path"})
				}
			}
		case *ast.BinaryExpr:
			if item.Op == token.ADD && isStringType(pass.TypesInfo.TypeOf(item)) {
				risks = append(risks, hotRisk{rule: "performance-frame-allocation", pos: item.Pos(), end: item.End(), message: "string construction is reachable on a frame path"})
			}
		}
		return true
	})
	return uniqueHotCalls(calls), risks
}

func walkHotPaths(root hotRoot, functions map[*types.Func]*hotFunction, result performanceFindings) {
	type pending struct {
		function *hotFunction
		path     []string
	}
	queue := []pending{{root.function, []string{root.function.name}}}
	seen := make(map[*hotFunction]bool)
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current.function] {
			continue
		}
		seen[current.function] = true
		for _, risk := range current.function.risks {
			id := risk.rule
			message := risk.message
			if root.audio {
				id = "performance-audio-callback-risk"
				message = "audio callback reaches structural risk requiring underrun and allocation measurement"
			}
			message += ": " + strings.Join(current.path, " -> ")
			result[id] = append(result[id], analysis.Diagnostic{Pos: risk.pos, End: risk.end, Message: message, Related: []analysis.RelatedInformation{{Pos: root.function.pos, Message: "hot-path root"}}})
		}
		for _, callee := range current.function.calls {
			if next := functions[callee]; next != nil {
				queue = append(queue, pending{next, append(append([]string(nil), current.path...), next.name)})
			}
		}
	}
}

func callbackRegistration(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	callee := staticHotCallee(pass, call)
	if callee == nil || callee.Pkg() == nil || callee.Pkg().Path() != playdatePackage {
		return "", false
	}
	switch callee.Name() {
	case "NewPCMCallbackSource", "NewGeneratorSynth":
		return callee.Name(), true
	case "SetDrawCallback", "SetUpdateCallback", "SetCollisionResponseCallback", "SetFinishCallback", "SetLoopCallback":
		return callee.Name(), false
	default:
		return "", false
	}
}

func isGameUpdate(function *types.Func) bool {
	signature := function.Type().(*types.Signature)
	if function.Name() != "Update" || signature.Recv() == nil || signature.Params().Len() != 1 || signature.Results().Len() != 2 {
		return false
	}
	return sdkNamed(signature.Params().At(0).Type(), playdatePackage, "Context") &&
		isBasicKind(signature.Results().At(0).Type(), types.Bool) && isBuiltinError(signature.Results().At(1).Type())
}

func isBasicKind(value types.Type, kind types.BasicKind) bool {
	basic, ok := value.Underlying().(*types.Basic)
	return ok && basic.Kind() == kind
}
func isBuiltinError(value types.Type) bool {
	return types.Identical(value, types.Universe.Lookup("error").Type())
}

func boundedForLoop(pass *analysis.Pass, loop *ast.ForStmt) bool {
	if loop.Cond == nil {
		return false
	}
	condition, ok := loop.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.LSS && condition.Op != token.LEQ && condition.Op != token.GTR && condition.Op != token.GEQ {
		return false
	}
	if value, ok := pass.TypesInfo.Types[condition.Y]; !ok || value.Value == nil {
		return false
	}
	switch loop.Post.(type) {
	case *ast.IncDecStmt, *ast.AssignStmt:
		return loop.Init != nil
	default:
		return false
	}
}

func staticHotCallee(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		object, _ := pass.TypesInfo.Uses[fun].(*types.Func)
		return object
	case *ast.SelectorExpr:
		if selection := pass.TypesInfo.Selections[fun]; selection != nil {
			object, _ := selection.Obj().(*types.Func)
			return object
		}
		object, _ := pass.TypesInfo.Uses[fun.Sel].(*types.Func)
		return object
	default:
		return nil
	}
}

func packageAndName(function *types.Func) (string, string) {
	if function.Pkg() == nil {
		return "", function.Name()
	}
	return function.Pkg().Path(), function.Name()
}
func hotFunctionName(function *types.Func) string {
	if sig := function.Type().(*types.Signature); sig.Recv() != nil {
		return fmt.Sprintf("%s.%s", types.TypeString(sig.Recv().Type(), func(*types.Package) string { return "" }), function.Name())
	}
	return function.Name()
}
func isStringType(value types.Type) bool {
	if value == nil {
		return false
	}
	basic, ok := value.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.String
}
func isResourceOrIOCall(path, name string) bool {
	return strings.HasPrefix(name, "Load") || name == "Open" || name == "Read" || name == "Write" || strings.HasPrefix(path, "net") || path == "os"
}
func isBlockingCall(path, name string) bool {
	return (path == "time" && name == "Sleep") || (path == "sync" && (name == "Lock" || name == "Wait"))
}
func uniqueHotCalls(source []*types.Func) []*types.Func {
	seen := map[*types.Func]bool{}
	out := source[:0]
	for _, item := range source {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func loopGrowthRisks(pass *analysis.Pass, body *ast.BlockStmt) []hotRisk {
	var risks []hotRisk
	ast.Inspect(body, func(node ast.Node) bool {
		switch item := node.(type) {
		case *ast.DeferStmt:
			risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Defer, end: item.End(), message: "defer is registered from a loop reachable on a frame path"})
		case *ast.CallExpr:
			if identifier, ok := item.Fun.(*ast.Ident); ok && identifier.Name == "append" {
				risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: item.Pos(), end: item.End(), message: "slice growth in a loop has no statically proven capacity bound"})
			}
		case *ast.AssignStmt:
			for _, target := range item.Lhs {
				if index, ok := target.(*ast.IndexExpr); ok {
					if _, ok := pass.TypesInfo.TypeOf(index.X).Underlying().(*types.Map); ok {
						risks = append(risks, hotRisk{rule: "performance-frame-unbounded-work", pos: index.Pos(), end: index.End(), message: "map growth in a loop has no statically proven capacity bound"})
					}
				}
			}
		}
		return true
	})
	return risks
}
