package analyzer

import (
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const playdatePackage = "github.com/ivan-gromov-dev/gopdsdk/playdate"

type applicationFindings map[RuleID][]analysis.Diagnostic

func applicationRuleRegistrations() []Registration {
	providers := StandardProviders()
	provider := &analysis.Analyzer{Name: "applicationcontracts", Doc: "model locally proven application and callback contracts", Requires: []*analysis.Analyzer{providers.SSA}, ResultType: reflect.TypeOf(applicationFindings{}), Run: func(pass *analysis.Pass) (any, error) {
		result := make(applicationFindings)
		functions := pass.ResultOf[providers.SSA].(*buildssa.SSA).SrcFuncs
		checkApplicationShape(pass, functions, result)
		checkApplicationPaths(pass, functions, result)
		checkApplicationFields(pass, functions, result)
		return result, PassContext(pass).Err()
	}}
	var registrations []Registration
	for _, id := range []RuleID{"application-entry", "application-lifecycle-shape", "application-scheduler-update-boundary", "application-nested-stencil", "application-nested-scheduler", "application-sprite-callback-close", "application-termination-resource-leak", "lifetime-framebuffer-escape", "lifetime-bitmap-data-escape", "lifetime-microphone-samples-escape", "lifetime-audio-render-buffer-escape", "lifetime-framebuffer-possible-escape", "lifetime-bitmap-data-possible-escape", "lifetime-microphone-samples-possible-escape", "lifetime-audio-render-buffer-possible-escape"} {
		registrations = append(registrations, Registration{RuleID: id, Analyzer: &analysis.Analyzer{Name: "application_" + strings.ReplaceAll(string(id), "-", "_"), Doc: "report a proven application contract violation", Requires: []*analysis.Analyzer{provider}, Run: func(pass *analysis.Pass) (any, error) {
			for _, diagnostic := range pass.ResultOf[provider].(applicationFindings)[id] {
				pass.Report(diagnostic)
			}
			return nil, nil
		}}})
	}
	return registrations
}

func (result applicationFindings) add(id RuleID, pos token.Pos, message string) {
	if !pos.IsValid() {
		return
	}
	for _, previous := range result[id] {
		if previous.Pos == pos {
			return
		}
	}
	result[id] = append(result[id], analysis.Diagnostic{Pos: pos, Message: message})
}

func sdkNamed(t types.Type, path, name string) bool {
	if t == nil {
		return false
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == path && named.Obj().Name() == name
}

func sdkInterface(pass *analysis.Pass, name string) *types.Interface {
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() == playdatePackage {
			if object := pkg.Scope().Lookup(name); object != nil {
				value, _ := object.Type().Underlying().(*types.Interface)
				return value
			}
		}
	}
	return nil
}

func checkApplicationShape(pass *analysis.Pass, functions []*ssa.Function, result applicationFindings) {
	if len(pass.Files) == 0 {
		return
	}
	game := sdkInterface(pass, "Game")
	entry := false
	// pdxinfo is the build command's application marker. A library named game
	// or an arbitrary New function is not sufficient evidence of an entry.
	for _, file := range pass.Files {
		path := filepath.Join(filepath.Dir(pass.Fset.Position(file.Pos()).Filename), "pdxinfo")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			entry = true
			break
		}
	}
	if entry {
		object := pass.Pkg.Scope().Lookup("New")
		valid := false
		pos := pass.Files[0].Name.Pos()
		if object != nil {
			pos = object.Pos()
			if function, ok := object.(*types.Func); ok {
				sig := function.Type().(*types.Signature)
				valid = sig.Recv() == nil && sig.Params().Len() == 0 && sig.Results().Len() == 1 && sig.TypeParams().Len() == 0 && sdkNamed(sig.Results().At(0).Type(), playdatePackage, "Game")
				if valid {
					_, valid = sig.Results().At(0).Type().Underlying().(*types.Interface)
				}
			}
		}
		if !valid || pass.Pkg.Name() == "main" {
			result.add("application-entry", pos, "application with pdxinfo must be importable and declare func New() playdate.Game")
		}
	}
	if game == nil {
		return
	}
	lifecycle := sdkInterface(pass, "LifecycleGame")
	if lifecycle == nil {
		return
	}
	for _, function := range functions {
		if function.Name() != "New" || function.Signature.Recv() != nil {
			continue
		}
		visitFactoryReturns(function, 0, make(map[*ssa.Function]bool), func(value ssa.Value) {
			boxed, ok := value.(*ssa.MakeInterface)
			if !ok || !sdkNamed(boxed.Type(), playdatePackage, "Game") {
				return
			}
			typ := boxed.X.Type()
			if !types.Implements(typ, game) || types.Implements(typ, lifecycle) {
				return
			}
			method, _, _ := types.LookupFieldOrMethod(typ, true, pass.Pkg, "HandleLifecycle")
			if method != nil {
				result.add("application-lifecycle-shape", method.Pos(), "HandleLifecycle does not implement playdate.LifecycleGame for this Game value; runtime lifecycle events cannot reach it (check signature and pointer/value receiver)")
			}
		})
	}
}

func visitFactoryReturns(function *ssa.Function, depth int, seen map[*ssa.Function]bool, visit func(ssa.Value)) {
	if function == nil || depth > 8 || seen[function] {
		return
	}
	seen[function] = true
	values := make(map[ssa.Value]bool)
	var walk func(ssa.Value)
	walk = func(value ssa.Value) {
		if values[value] {
			return
		}
		values[value] = true
		visit(value)
		switch value := value.(type) {
		case *ssa.Phi:
			for _, edge := range value.Edges {
				walk(edge)
			}
		case *ssa.Call:
			callee := value.Common().StaticCallee()
			if callee != nil && callee.Pkg == function.Pkg {
				visitFactoryReturns(callee, depth+1, seen, visit)
			}
		}
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			if ret, ok := instruction.(*ssa.Return); ok {
				for _, value := range ret.Results {
					walk(value)
				}
			}
		}
	}
}

// callMethod uses package identity, including interface invocation, rather than
// spelling alone. User methods named Update, Close, or WithStencil are unrelated.
func callMethod(call *ssa.CallCommon, path, receiver, name string) bool {
	if call.IsInvoke() {
		if call.Method.Name() != name {
			return false
		}
		if sdkNamed(call.Value.Type(), path, receiver) {
			return true
		}
		signature, _ := call.Method.Type().(*types.Signature)
		return signature != nil && signature.Recv() != nil && sdkNamed(signature.Recv().Type(), path, receiver)
	}
	callee := call.StaticCallee()
	return callee != nil && callee.Name() == name && callee.Signature.Recv() != nil && sdkNamed(callee.Signature.Recv().Type(), path, receiver)
}

func callbackFunction(value ssa.Value) *ssa.Function {
	switch value := value.(type) {
	case *ssa.Function:
		return value
	case *ssa.MakeClosure:
		function, _ := value.Fn.(*ssa.Function)
		return function
	case *ssa.ChangeType:
		return callbackFunction(value.X)
	}
	return nil
}

func checkApplicationPaths(pass *analysis.Pass, functions []*ssa.Function, result applicationFindings) {
	game := sdkInterface(pass, "Game")
	for _, function := range functions {
		if PassContext(pass).Err() != nil {
			return
		}
		if game != nil && function.Signature.Recv() != nil && types.Implements(function.Signature.Recv().Type(), game) && (function.Name() == "Init" || function.Name() == "HandleLifecycle") {
			checkTerminationResources(function, result)
			walkApplicationCalls(function, 0, make(map[*ssa.Function]bool), func(call *ssa.CallCommon) {
				if callMethod(call, playdatePackage+"/schedule", "Scheduler", "Update") {
					result.add("application-scheduler-update-boundary", call.Pos(), "Scheduler.Update is reachable from Game."+function.Name()+"; advance the scheduler only from Game.Update")
				}
			})
		}
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				callInstruction, ok := instruction.(*ssa.Call)
				if !ok {
					continue
				}
				call := callInstruction.Common()

				for _, argument := range call.Args {
					callback := callbackFunction(argument)
					if callback == nil {
						continue
					}
					if nativeApplicationCallback(call) {
						walkApplicationCalls(callback, 0, make(map[*ssa.Function]bool), func(nested *ssa.CallCommon) {
							if callMethod(nested, playdatePackage+"/schedule", "Scheduler", "Update") {
								result.add("application-scheduler-update-boundary", nested.Pos(), "Scheduler.Update runs from a native callback; advance it from Game.Update")
							}
						})
					}
					if callMethod(call, playdatePackage, "BitmapCompositor", "WithStencil") {
						walkApplicationCalls(callback, 0, make(map[*ssa.Function]bool), func(nested *ssa.CallCommon) {
							if callMethod(nested, playdatePackage, "BitmapCompositor", "WithStencil") {
								result.add("application-nested-stencil", nested.Pos(), "WithStencil is called during an active stencil callback; stencil callbacks cannot be nested")
							}
						})
					}
					for _, method := range []string{"SetUpdateCallback", "SetDrawCallback", "SetCollisionResponseCallback"} {
						if callMethod(call, playdatePackage, "Sprite", method) {
							for _, parameter := range callback.Params {
								if sdkNamed(parameter.Type(), playdatePackage, "Sprite") {
									checkScopedValue(parameter, "application-sprite-callback-close", result, make(map[ssa.Value]bool), parameter.Parent())
								}
							}
						}
					}
					for _, parameter := range callback.Params {
						id := scopedCallbackRule(call, parameter)
						if id != "" {
							checkScopedValue(parameter, id, result, make(map[ssa.Value]bool), parameter.Parent())
						}
					}
					// A task can only execute during its scheduler's Update. Restrict
					// re-entry to an identical receiver captured by a closure.
					for _, method := range []string{"Schedule", "ScheduleAfter", "ScheduleAt"} {
						if callMethod(call, playdatePackage+"/schedule", "Scheduler", method) {
							if converted, ok := argument.(*ssa.ChangeType); ok {
								argument = converted.X
							}
							if closure, ok := argument.(*ssa.MakeClosure); ok && len(call.Args) > 0 {
								for index, binding := range closure.Bindings {
									if sameCapturedReceiver(binding, call.Args[0]) && index < len(callback.FreeVars) {
										checkSchedulerReentry(callback.FreeVars[index], result)
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func nativeApplicationCallback(call *ssa.CallCommon) bool {
	for _, operation := range []struct{ receiver, method string }{
		{"Sprite", "SetUpdateCallback"}, {"Sprite", "SetDrawCallback"}, {"Sprite", "SetCollisionResponseCallback"},
		{"CallbackAudio", "NewPCMCallbackSource"}, {"GeneratorSynthesizers", "NewGeneratorSynth"}, {"Microphones", "StartMicrophoneRecording"},
	} {
		if callMethod(call, playdatePackage, operation.receiver, operation.method) {
			return true
		}
	}
	return false
}

func scopedCallbackRule(call *ssa.CallCommon, parameter *ssa.Parameter) RuleID {
	for _, entry := range []struct {
		receiver, method, typ string
		id                    RuleID
	}{
		{"FramebufferGraphics", "WithFramebuffer", "Framebuffer", "lifetime-framebuffer-escape"},
		{"BitmapDataGraphics", "WithBitmapData", "BitmapData", "lifetime-bitmap-data-escape"},
		{"Microphones", "StartMicrophoneRecording", "MicrophoneSamples", "lifetime-microphone-samples-escape"},
	} {
		if callMethod(call, playdatePackage, entry.receiver, entry.method) && sdkNamed(parameter.Type(), playdatePackage, entry.typ) {
			return entry.id
		}
	}
	if callMethod(call, playdatePackage, "CallbackAudio", "NewPCMCallbackSource") || callMethod(call, playdatePackage, "GeneratorSynthesizers", "NewGeneratorSynth") {
		if slice, ok := parameter.Type().Underlying().(*types.Slice); ok && types.Identical(slice.Elem(), types.Typ[types.Int16]) {
			return "lifetime-audio-render-buffer-escape"
		}
	}
	return ""
}

func checkSchedulerReentry(value ssa.Value, result applicationFindings) {
	if value.Referrers() == nil {
		return
	}
	for _, ref := range *value.Referrers() {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == value {
			return
		}
	}
	for _, ref := range *value.Referrers() {
		if load, ok := ref.(*ssa.UnOp); ok && load.Op == token.MUL {
			checkSchedulerReentry(load, result)
		}
		if call, ok := ref.(*ssa.Call); ok && callMethod(call.Common(), playdatePackage+"/schedule", "Scheduler", "Update") && len(call.Common().Args) > 0 && call.Common().Args[0] == value {
			result.add("application-nested-scheduler", call.Pos(), "Scheduler.Update re-enters the same scheduler from its scheduled task")
		}
	}
}

func sameCapturedReceiver(binding, receiver ssa.Value) bool {
	if binding == receiver {
		return true
	}
	load, ok := receiver.(*ssa.UnOp)
	if !ok || load.Op != token.MUL || load.X != binding || binding.Referrers() == nil {
		return false
	}
	stores := 0
	for _, ref := range *binding.Referrers() {
		switch ref := ref.(type) {
		case *ssa.Store:
			if ref.Addr != binding {
				return false
			}
			stores++
		case *ssa.UnOp:
		case *ssa.MakeClosure:
		case *ssa.DebugRef:
		default:
			return false
		}
	}
	return stores == 1
}

// Propagate only identity-preserving operations. Copies, unknown calls, joins,
// and heap aliases are deliberately not treated as proof of an escape.
func checkScopedValue(value ssa.Value, id RuleID, result applicationFindings, seen map[ssa.Value]bool, boundary *ssa.Function) {
	if seen[value] || value.Referrers() == nil {
		return
	}
	seen[value] = true
	for _, ref := range *value.Referrers() {
		switch ref := ref.(type) {
		case *ssa.ChangeType:
			checkScopedValue(ref, id, result, seen, boundary)
		case *ssa.Convert:
			if _, ok := ref.Type().Underlying().(*types.Slice); ok {
				checkScopedValue(ref, id, result, seen, boundary)
			}
		case *ssa.MakeInterface:
			checkScopedValue(ref, id, result, seen, boundary)
		case *ssa.Slice:
			checkScopedValue(ref, id, result, seen, boundary)
		case *ssa.Phi:
			checkScopedValue(ref, id, result, seen, boundary)
		case *ssa.MakeClosure:
			if closureEscapes(ref) {
				result.add(id, ref.Pos(), "callback-scoped value is captured by a closure that outlives the callback; copy required data before capture")
			}
		case *ssa.Return:
			if ref.Parent() == boundary {
				result.add(id, ref.Pos(), "callback-scoped value is returned from its callback; copy required data before returning")
			}
		case *ssa.MapUpdate:
			if ref.Value == value || ref.Key == value {
				result.add(id, ref.Pos(), "callback-scoped value is inserted into a map; copy required data before insertion")
			}
		case *ssa.Send:
			if ref.X == value {
				result.add(id, ref.Pos(), "callback-scoped value is sent on a channel; copy required data before sending")
			}
		case *ssa.Go:
			if argumentOf(ref.Common(), value) {
				result.add(id, ref.Pos(), "callback-scoped value is passed to a goroutine that can outlive the callback; copy required data before starting it")
			}
		case *ssa.Store:
			if ref.Val != value || id == "application-sprite-callback-close" {
				continue
			}
			if callbackExternalAddress(ref.Addr) {
				result.add(id, ref.Pos(), "callback-scoped value is retained outside the callback; copy required data before the callback returns")
			}
		case *ssa.Call:
			call := ref.Common()
			if builtin, ok := call.Value.(*ssa.Builtin); ok && builtin.Name() == "append" {
				// append into a nil slice is the standard explicit slice copy.
				// Other destinations may reuse storage that aliases callback data.
				fresh := false
				if len(call.Args) > 0 {
					constant, ok := call.Args[0].(*ssa.Const)
					fresh = ok && constant.IsNil()
				}
				if !fresh {
					checkScopedValue(ref, id, result, seen, boundary)
				}
				continue
			}
			if callee := call.StaticCallee(); callee != nil && callee.Pkg == ref.Parent().Pkg && len(seen) < 128 {
				for index, argument := range call.Args {
					if argument == value && index < len(callee.Params) {
						checkScopedValue(callee.Params[index], id, result, seen, boundary)
						if helperReturnsParameter(callee, callee.Params[index]) {
							checkScopedValue(ref, id, result, seen, boundary)
						}
					}
				}
			} else if argumentOf(call, value) && !scopedReadOnlyCall(call, value) {
				result.add(possibleLifetimeRule(id), ref.Pos(), "callback-scoped value is passed to a call whose retention behavior is unknown")
			}
			if id == "application-sprite-callback-close" {
				if call.IsInvoke() && call.Value == value && callMethod(call, playdatePackage, "Sprite", "Close") {
					result.add(id, ref.Pos(), "cannot close a sprite passed to its active sprite callback; defer cleanup until after callback delivery")
				}
				continue
			}
			if call.IsInvoke() && call.Value == value && (call.Method.Name() == "Bytes" || call.Method.Name() == "MaskBytes") {
				if ref.Referrers() != nil {
					for _, use := range *ref.Referrers() {
						if extract, ok := use.(*ssa.Extract); ok && extract.Index == 0 {
							checkScopedValue(extract, id, result, seen, boundary)
						}
					}
				}
			}
		}
	}
}

func helperReturnsParameter(function *ssa.Function, parameter *ssa.Parameter) bool {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok {
				continue
			}
			for _, value := range returned.Results {
				if value == parameter {
					return true
				}
			}
		}
	}
	return false
}

func closureEscapes(closure *ssa.MakeClosure) bool {
	if closure.Referrers() == nil {
		return false
	}
	for _, ref := range *closure.Referrers() {
		switch ref := ref.(type) {
		case *ssa.Return, *ssa.Go, *ssa.Send, *ssa.MapUpdate:
			return true
		case *ssa.Store:
			if ref.Val == closure && callbackExternalAddress(ref.Addr) {
				return true
			}
		case *ssa.Call:
			if ref.Common().Value != closure {
				return true
			}
		}
	}
	return false
}

func argumentOf(call *ssa.CallCommon, value ssa.Value) bool {
	for _, argument := range call.Args {
		if argument == value {
			return true
		}
	}
	return false
}

func scopedReadOnlyCall(call *ssa.CallCommon, value ssa.Value) bool {
	if builtin, ok := call.Value.(*ssa.Builtin); ok {
		return builtin.Name() == "len" || builtin.Name() == "cap" || builtin.Name() == "copy"
	}
	return call.IsInvoke() && call.Value == value
}

func possibleLifetimeRule(id RuleID) RuleID {
	return RuleID(strings.TrimSuffix(string(id), "-escape") + "-possible-escape")
}

func callbackExternalAddress(value ssa.Value) bool {
	switch value := value.(type) {
	case *ssa.Global, *ssa.FreeVar, *ssa.Parameter:
		return true
	case *ssa.Alloc:
		return value.Heap
	case *ssa.FieldAddr:
		return callbackExternalAddress(value.X)
	case *ssa.IndexAddr:
		return callbackExternalAddress(value.X)
	case *ssa.UnOp:
		return value.Op == token.MUL && callbackExternalAddress(value.X)
	}
	return false
}
