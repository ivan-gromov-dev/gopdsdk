package analyzer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

// deviceRuleRegistrations returns the fast, intraprocedural rules that enforce
// the documented device Go subset. Each rule has its own analyzer identity so
// callers can select and suppress stable rule IDs independently.
func deviceRuleRegistrations() []Registration {
	providers := StandardProviders()
	reachability := newDeviceReachabilityProvider()
	return []Registration{
		{RuleID: "device-goroutine", Analyzer: syntaxRule("devicegoroutine", providers, func(pass *analysis.Pass, node ast.Node) {
			if statement, ok := node.(*ast.GoStmt); ok {
				pass.Reportf(statement.Go, "goroutines are unavailable on device; use playdate/schedule")
			}
		})},
		{RuleID: "device-channel", Analyzer: syntaxRule("devicechannel", providers, reportChannel)},
		{RuleID: "device-select", Analyzer: syntaxRule("deviceselect", providers, func(pass *analysis.Pass, node ast.Node) {
			if statement, ok := node.(*ast.SelectStmt); ok {
				pass.Reportf(statement.Select, "select is unavailable on device; use bounded deterministic scheduling")
			}
		})},
		{RuleID: "device-time-runtime", Analyzer: syntaxRule("devicetimeruntime", providers, reportTimeUse)},
		{RuleID: "device-encoding-json", Analyzer: deviceReachabilityRule("deviceencodingjson", reachability, "device-encoding-json")},
		{RuleID: "device-fmt", Analyzer: deviceReachabilityRule("devicefmt", reachability, "device-fmt")},
		{RuleID: "device-recover", Analyzer: callRule("devicerecover", providers, func(pass *analysis.Pass, call *ast.CallExpr) {
			if objectName(pass, call.Fun, "builtin", "recover") {
				pass.Reportf(call.Fun.Pos(), "recover cannot recover a device panic")
			}
		})},
		{RuleID: "device-finalizer", Analyzer: syntaxRule("devicefinalizer", providers, func(pass *analysis.Pass, node ast.Node) {
			if selector, ok := node.(*ast.SelectorExpr); ok && objectName(pass, selector, "runtime", "SetFinalizer") {
				pass.Reportf(selector.Pos(), "runtime.SetFinalizer is unavailable on device; use explicit ownership")
			}
		})},
		{RuleID: "device-cgo", Analyzer: importRule("devicecgo", providers, "C", "application cgo is unavailable on device")},
		{RuleID: "device-reflect-symbol", Analyzer: syntaxRule("devicereflectsymbol", providers, reportReflectUse)},
		{RuleID: "device-runtime-control", Analyzer: syntaxRule("deviceruntimecontrol", providers, reportRuntimeControlUse)},
	}
}

type nodeReporter func(*analysis.Pass, ast.Node)
type callReporter func(*analysis.Pass, *ast.CallExpr)

func syntaxRule(name string, providers Providers, report nodeReporter) *analysis.Analyzer {
	return &analysis.Analyzer{Name: name, Doc: "check the gopdsdk device Go subset", Requires: []*analysis.Analyzer{providers.Syntax}, Run: func(pass *analysis.Pass) (any, error) {
		inspect := pass.ResultOf[providers.Syntax].(*inspector.Inspector)
		inspect.Preorder(nil, func(node ast.Node) { report(pass, node) })
		return nil, nil
	}}
}

func callRule(name string, providers Providers, report callReporter) *analysis.Analyzer {
	return syntaxRule(name, providers, func(pass *analysis.Pass, node ast.Node) {
		if call, ok := node.(*ast.CallExpr); ok {
			report(pass, call)
		}
	})
}

func importRule(name string, providers Providers, path, message string) *analysis.Analyzer {
	return syntaxRule(name, providers, func(pass *analysis.Pass, node ast.Node) {
		file, ok := node.(*ast.File)
		if !ok {
			return
		}
		for _, spec := range file.Imports {
			if spec.Path.Value == `"`+path+`"` {
				pass.Reportf(spec.Path.Pos(), "%s", message)
			}
		}
	})
}

func reportChannel(pass *analysis.Pass, node ast.Node) {
	switch value := node.(type) {
	case *ast.ChanType:
		pass.Reportf(value.Begin, "channel types are unavailable on device; use playdate/schedule")
	case *ast.SendStmt:
		pass.Reportf(value.Arrow, "channel sends are unavailable on device; use playdate/schedule")
	case *ast.UnaryExpr:
		if value.Op.String() == "<-" {
			pass.Reportf(value.OpPos, "channel receives are unavailable on device; use playdate/schedule")
		}
	case *ast.CallExpr:
		if objectName(pass, value.Fun, "builtin", "close") && len(value.Args) == 1 && isChannel(pass.TypesInfo.TypeOf(value.Args[0])) {
			pass.Reportf(value.Fun.Pos(), "channel operations are unavailable on device; use playdate/schedule")
		}
	case *ast.RangeStmt:
		if isChannel(pass.TypesInfo.TypeOf(value.X)) {
			pass.Reportf(value.For, "ranging over channels is unavailable on device; use playdate/schedule")
		}
	}
}

func reportTimeUse(pass *analysis.Pass, node ast.Node) {
	selector, ok := node.(*ast.SelectorExpr)
	if !ok {
		return
	}
	object := pass.TypesInfo.ObjectOf(selector.Sel)
	if object == nil || object.Pkg() == nil || object.Pkg().Path() != "time" {
		return
	}
	if allowedTimeSymbol(object) {
		return
	}
	pass.Reportf(selector.Pos(), "time.%s uses clocks or runtime scheduling unavailable on device; use playdate/schedule", object.Name())
}

func allowedTimeSymbol(object types.Object) bool {
	switch object.Name() {
	case "Duration", "ParseDuration", "Nanosecond", "Microsecond", "Millisecond", "Second", "Minute", "Hour":
		return true
	}
	function, ok := object.(*types.Func)
	if !ok || function.Type().(*types.Signature).Recv() == nil {
		return false
	}
	switch object.Name() {
	case "String", "Abs", "Hours", "Minutes", "Seconds", "Milliseconds", "Microseconds", "Nanoseconds", "Round", "Truncate":
		return isNamedType(function.Type().(*types.Signature).Recv().Type(), "time", "Duration")
	default:
		return false
	}
}

func reportReflectUse(pass *analysis.Pass, node ast.Node) {
	selector, ok := node.(*ast.SelectorExpr)
	if !ok {
		return
	}
	object := pass.TypesInfo.ObjectOf(selector.Sel)
	if object == nil || object.Pkg() == nil || object.Pkg().Path() != "reflect" || allowedReflectCall(object) {
		return
	}
	pass.Reportf(selector.Pos(), "reflect.%s is outside the audited device reflection subset", object.Name())
}

func allowedReflectCall(object types.Object) bool {
	function, ok := object.(*types.Func)
	if !ok {
		return true
	}
	signature := function.Type().(*types.Signature)
	if signature.Recv() == nil {
		return object.Name() == "TypeOf" || object.Name() == "ValueOf"
	}
	switch object.Name() {
	case "Field", "Kind", "Name", "NumField":
		return isNamedType(signature.Recv().Type(), "reflect", "Type") || isNamedType(signature.Recv().Type(), "reflect", "Value")
	case "CanInterface", "CanSet", "Convert", "Elem", "Index", "Int", "Interface", "IsValid", "MapIndex", "SetInt", "SetMapIndex", "SetString":
		return isNamedType(signature.Recv().Type(), "reflect", "Value")
	case "Get", "Lookup":
		return isNamedType(signature.Recv().Type(), "reflect", "StructTag")
	default:
		return false
	}
}

func reportRuntimeControlUse(pass *analysis.Pass, node ast.Node) {
	selector, ok := node.(*ast.SelectorExpr)
	if !ok {
		return
	}
	object := pass.TypesInfo.ObjectOf(selector.Sel)
	if object == nil || object.Pkg() == nil || object.Pkg().Path() != "runtime" {
		return
	}
	switch object.Name() {
	case "LockOSThread", "Goexit", "GOMAXPROCS":
	default:
		return
	}
	pass.Reportf(selector.Pos(), "runtime.%s is an application runtime-control hook unavailable on device", object.Name())
}

func calledObject(pass *analysis.Pass, expression ast.Expr) types.Object {
	switch expression := expression.(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(expression)
	case *ast.SelectorExpr:
		return pass.TypesInfo.ObjectOf(expression.Sel)
	default:
		return nil
	}
}

func objectName(pass *analysis.Pass, expression ast.Expr, packagePath, name string) bool {
	object := calledObject(pass, expression)
	if object == nil || object.Name() != name {
		return false
	}
	if packagePath == "builtin" {
		return object.Pkg() == nil
	}
	return object.Pkg() != nil && object.Pkg().Path() == packagePath
}

func isChannel(value types.Type) bool {
	if value == nil {
		return false
	}
	_, ok := value.Underlying().(*types.Chan)
	return ok
}

func isNamedType(value types.Type, packagePath, name string) bool {
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	named, ok := value.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == name
}
