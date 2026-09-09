package analyzer

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const sdkPackagePrefix = "github.com/ivan-gromov-dev/gopdsdk/playdate"

type resultFindings map[RuleID][]analysis.Diagnostic

func resultRuleRegistrations() []Registration {
	provider := &analysis.Analyzer{Name: "sdkresultcontracts", Doc: "model SDK-specific errors, results, and statically proven values", Requires: []*analysis.Analyzer{buildssa.Analyzer}, ResultType: reflect.TypeOf(resultFindings{}), Run: func(pass *analysis.Pass) (any, error) {
		result := make(resultFindings)
		for _, function := range pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA).SrcFuncs {
			checkResultFunction(function, result)
		}
		return result, PassContext(pass).Err()
	}}
	ids := []RuleID{"result-sdk-error-discarded", "result-sdk-value-discarded", "result-sdk-error-comparison", "result-sdk-typed-diagnostic", "result-invalid-argument", "result-menu-image-offset"}
	registrations := make([]Registration, 0, len(ids))
	for _, id := range ids {
		registrations = append(registrations, Registration{RuleID: id, Analyzer: &analysis.Analyzer{Name: "sdk_" + strings.ReplaceAll(string(id), "-", "_"), Doc: "report a proven SDK result contract violation", Requires: []*analysis.Analyzer{provider}, Run: func(pass *analysis.Pass) (any, error) {
			for _, diagnostic := range pass.ResultOf[provider].(resultFindings)[id] {
				pass.Report(diagnostic)
			}
			return nil, nil
		}}})
	}
	return registrations
}

func (result resultFindings) add(id RuleID, pos token.Pos, message string) {
	if !pos.IsValid() {
		return
	}
	for _, diagnostic := range result[id] {
		if diagnostic.Pos == pos && diagnostic.Message == message {
			return
		}
	}
	result[id] = append(result[id], analysis.Diagnostic{Pos: pos, Message: message})
}

func checkResultFunction(function *ssa.Function, result resultFindings) {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			switch instruction := instruction.(type) {
			case *ssa.Call:
				checkSDKCallResults(instruction, result)
				checkSDKArguments(instruction.Common(), instruction.Pos(), result)
			case *ssa.Defer:
				checkSDKCallResults(instruction, result)
				checkSDKArguments(instruction.Common(), instruction.Pos(), result)
			case *ssa.Go:
				checkSDKCallResults(instruction, result)
				checkSDKArguments(instruction.Common(), instruction.Pos(), result)
			case *ssa.BinOp:
				if (instruction.Op == token.EQL || instruction.Op == token.NEQ) && (sdkSentinel(instruction.X) || sdkSentinel(instruction.Y)) {
					result.add("result-sdk-error-comparison", instruction.Pos(), "wrapped gopdsdk sentinels must be matched with errors.Is, not direct equality")
				}
			case *ssa.TypeAssert:
				if isErrorType(instruction.X.Type()) && sdkDiagnosticType(instruction.AssertedType) {
					result.add("result-sdk-typed-diagnostic", instruction.Pos(), "wrapped gopdsdk typed diagnostics must be extracted with errors.As, not a direct type assertion")
				}
			}
		}
	}
}

type callInstruction interface {
	ssa.Instruction
	Common() *ssa.CallCommon
}

func checkSDKCallResults(instruction callInstruction, result resultFindings) {
	call := instruction.Common()
	path, receiver, name, signature := sdkCall(call)
	if !strings.HasPrefix(path, sdkPackagePrefix) || signature == nil {
		return
	}
	used := make(map[int]bool)
	if value, ok := instruction.(ssa.Value); ok && value.Referrers() != nil {
		for _, ref := range *value.Referrers() {
			if extract, ok := ref.(*ssa.Extract); ok {
				used[extract.Index] = true
			} else if signature.Results().Len() == 1 {
				used[0] = true
			}
		}
	}
	for index := 0; index < signature.Results().Len(); index++ {
		if used[index] {
			continue
		}
		typ := signature.Results().At(index).Type()
		if isErrorType(typ) {
			result.add("result-sdk-error-discarded", instruction.Pos(), fmt.Sprintf("error result %d from %s must be observed; it may carry a gopdsdk sentinel or typed diagnostic", index+1, sdkCallName(path, receiver, name)))
		} else if significantResult(path, receiver, name, index, typ) {
			result.add("result-sdk-value-discarded", instruction.Pos(), fmt.Sprintf("result %d from %s is contract-significant and must be observed", index+1, sdkCallName(path, receiver, name)))
		}
	}
}

func significantResult(path, receiver, name string, index int, typ types.Type) bool {
	key := receiver + "." + name
	if receiver == "" {
		key = name
	}
	indices := map[string]map[int]bool{
		"Queue.TrySend": {0: true}, "Queue.TryReceive": {1: true}, "Poller.TryReceive": {1: true, 2: true},
		"Scheduler.Cancel": {0: true}, "Scheduler.Pending": {0: true}, "SystemControls.ButtonCallbackOverflow": {0: true},
		"TileMap.TileAt": {1: true}, "Value.Lookup": {1: true},
		"MicrophoneSamples.CopyTo": {0: true}, "PCMCallbackSource.CopyTo": {0: true},
	}
	return strings.HasPrefix(path, sdkPackagePrefix) && indices[key][index] && types.Identical(typ, types.Typ[types.Bool]) || indices[key][index]
}

func sdkCall(call *ssa.CallCommon) (path, receiver, name string, signature *types.Signature) {
	signature = call.Signature()
	if call.IsInvoke() {
		name = call.Method.Name()
		receiver = namedTypeName(call.Value.Type())
		if object := namedTypeObject(call.Value.Type()); object != nil && object.Pkg() != nil {
			path = object.Pkg().Path()
		}
		return
	}
	callee := call.StaticCallee()
	if callee == nil {
		return "", "", "", signature
	}
	origin := callee.Origin()
	if origin == nil {
		origin = callee
	}
	if origin.Pkg == nil || origin.Pkg.Pkg == nil {
		return "", "", "", signature
	}
	path, name = origin.Pkg.Pkg.Path(), origin.Name()
	if origin.Signature.Recv() != nil {
		receiver = namedTypeName(origin.Signature.Recv().Type())
	}
	return
}

func sdkCallName(path, receiver, name string) string {
	pkg := strings.TrimPrefix(path, "github.com/ivan-gromov-dev/gopdsdk/")
	if receiver != "" {
		return pkg + "." + receiver + "." + name
	}
	return pkg + "." + name
}

func namedTypeObject(typ types.Type) *types.TypeName {
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, _ := types.Unalias(typ).(*types.Named)
	if named == nil {
		return nil
	}
	return named.Obj()
}

func namedTypeName(typ types.Type) string {
	if object := namedTypeObject(typ); object != nil {
		return object.Name()
	}
	return ""
}

func isErrorType(typ types.Type) bool {
	return types.Implements(typ, types.Universe.Lookup("error").Type().Underlying().(*types.Interface))
}

func sdkDiagnosticType(typ types.Type) bool {
	object := namedTypeObject(typ)
	return object != nil && object.Exported() && object.Pkg() != nil && strings.HasPrefix(object.Pkg().Path(), sdkPackagePrefix) && isErrorType(typ)
}

func sdkSentinel(value ssa.Value) bool {
	switch value := value.(type) {
	case *ssa.UnOp:
		return sdkSentinel(value.X)
	case *ssa.ChangeType:
		return sdkSentinel(value.X)
	case *ssa.MakeInterface:
		return sdkSentinel(value.X)
	case *ssa.Global:
		return value.Pkg != nil && value.Pkg.Pkg != nil && strings.HasPrefix(value.Pkg.Pkg.Path(), sdkPackagePrefix) && strings.HasPrefix(value.Name(), "Err")
	}
	return false
}

func checkSDKArguments(call *ssa.CallCommon, pos token.Pos, result resultFindings) {
	path, receiver, name, _ := sdkCall(call)
	if !strings.HasPrefix(path, sdkPackagePrefix) {
		return
	}
	args := call.Args
	if !call.IsInvoke() && receiver != "" && len(args) > 0 {
		args = args[1:]
	}
	reportRange := func(index int, minimum, maximum int64, description string, id RuleID) {
		if index >= len(args) {
			return
		}
		low, high, ok := integerRange(args[index], make(map[ssa.Value]bool))
		if ok && (low < minimum || high > maximum) {
			result.add(id, pos, fmt.Sprintf("%s must be %s; proven range is %d..%d", sdkCallName(path, receiver, name), description, low, high))
		}
	}
	switch receiver + "." + name {
	case "SystemControls.SetMenuImage":
		reportRange(1, 0, 200, "between 0 and 200 inclusive", "result-menu-image-offset")
	case "SystemControls.SetButtonCallback":
		reportRange(1, 0, 64, "zero when disabled or between 1 and 64", "result-invalid-argument")
		if len(args) > 1 {
			low, high, known := integerRange(args[1], make(map[ssa.Value]bool))
			if callbackNil, nilKnown := nilValue(args[0]); known && nilKnown && ((callbackNil && (low != 0 || high != 0)) || (!callbackNil && low == 0 && high == 0)) {
				result.add("result-invalid-argument", pos, sdkCallName(path, receiver, name)+" requires a nil callback with queue size zero to disable delivery, or a non-nil callback with queue size 1..64")
			}
		}
	case ".New":
		if path == sdkPackagePrefix+"/diagnostics" {
			reportRange(0, 1, 36000, "between 1 and 36000 inclusive", "result-invalid-argument")
		}
	case ".NewQueue":
		reportRange(0, 1, math.MaxInt64, "positive", "result-invalid-argument")
	case ".NewAnimation":
		reportRange(1, 0, math.MaxInt64, "non-negative", "result-invalid-argument")
		reportRange(2, 1, math.MaxInt64, "positive", "result-invalid-argument")
	}
}

func nilValue(value ssa.Value) (bool, bool) {
	switch value := value.(type) {
	case *ssa.Const:
		return value.IsNil(), true
	case *ssa.ChangeType:
		return nilValue(value.X)
	case *ssa.MakeInterface:
		return false, true
	case *ssa.Function, *ssa.MakeClosure:
		return false, true
	}
	return false, false
}

func integerRange(value ssa.Value, seen map[ssa.Value]bool) (int64, int64, bool) {
	if seen[value] {
		return 0, 0, false
	}
	seen[value] = true
	switch value := value.(type) {
	case *ssa.Const:
		if value.Value == nil || value.Value.Kind() != constant.Int {
			return 0, 0, false
		}
		integer, ok := constant.Int64Val(value.Value)
		return integer, integer, ok
	case *ssa.Convert:
		return integerRange(value.X, seen)
	case *ssa.ChangeType:
		return integerRange(value.X, seen)
	case *ssa.Phi:
		var low, high int64
		for index, edge := range value.Edges {
			edgeLow, edgeHigh, ok := integerRange(edge, seen)
			if !ok {
				return 0, 0, false
			}
			if index == 0 || edgeLow < low {
				low = edgeLow
			}
			if index == 0 || edgeHigh > high {
				high = edgeHigh
			}
		}
		return low, high, len(value.Edges) != 0
	}
	return 0, 0, false
}
