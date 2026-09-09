package analyzer

import (
	"go/constant"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type capabilityFindings map[RuleID][]analysis.Diagnostic

type capabilityHelperFact struct {
	Parameter int
	Package   string
	Name      string
	Negated   bool
}

func (*capabilityHelperFact) AFact() {}

func capabilityRuleRegistrations() []Registration {
	providers := StandardProviders()
	provider := &analysis.Analyzer{Name: "capabilityfacts", Doc: "track checked optional capabilities by SSA value identity", Requires: []*analysis.Analyzer{providers.SSA}, ResultType: reflect.TypeOf(capabilityFindings{}), FactTypes: []analysis.Fact{new(capabilityHelperFact)}, Run: func(pass *analysis.Pass) (any, error) {
		findings := make(capabilityFindings)
		capabilities := loadedOptionalCapabilities(pass)
		functions := pass.ResultOf[providers.SSA].(*buildssa.SSA).SrcFuncs
		flow := newCapabilityFlow(pass, functions)
		exportCapabilityHelperFacts(pass, functions)
		checked := make(map[*ssa.Function][]types.Type)
		for _, function := range functions {
			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					if assertion, ok := instruction.(*ssa.TypeAssert); ok && assertion.CommaOk {
						checked[function] = append(checked[function], assertion.AssertedType)
					}
				}
			}
		}
		for _, function := range functions {
			if err := PassContext(pass).Err(); err != nil {
				return nil, err
			}
			for _, block := range function.Blocks {
				for _, instruction := range block.Instrs {
					assertion, ok := instruction.(*ssa.TypeAssert)
					if !ok || !optionalCapability(capabilities, assertion.AssertedType) {
						continue
					}
					known, present := capabilityAt(pass, assertion)
					if !known && !assertion.CommaOk && flow.present(assertion.X, assertion.AssertedType, assertion.Block(), make(map[capabilityQuery]bool), 0) {
						known, present = true, true
					}
					id := RuleID("")
					message := ""
					if assertion.CommaOk {
						if known {
							id = "capability-redundant-check"
							message = "capability check is already known to succeed on this path"
							if !present {
								message = "capability check is already known to fail on this path; use the fallback directly"
							}
						}
					} else if known && !present {
						id = "capability-impossible-assertion"
						message = "optional capability is unavailable on this path; this assertion will panic if reached"
					} else if !known && !capabilityInStaticInterface(assertion.X.Type(), assertion.AssertedType) {
						id = "capability-unchecked-assertion"
						message = "optional capability assertion has no proven successful check; use comma-ok and handle the unsupported case"
						for owner, targets := range checked {
							if owner == function || owner == function.Parent() {
								continue
							}
							for _, target := range targets {
								if types.Identical(target, assertion.AssertedType) {
									id = "capability-unproven-assertion"
									message = "a related capability check exists in another function, but no successful guard is proven here; pass the checked capability or check locally"
								}
							}
						}
					}
					if id != "" {
						findings[id] = append(findings[id], analysis.Diagnostic{Pos: assertion.Pos(), Message: message})
					}
				}
			}
		}
		return findings, nil
	}}
	var result []Registration
	for _, id := range []RuleID{"capability-unchecked-assertion", "capability-unproven-assertion", "capability-impossible-assertion", "capability-redundant-check"} {
		result = append(result, Registration{RuleID: id, Analyzer: &analysis.Analyzer{Name: strings.ReplaceAll(string(id), "-", "_"), Doc: "report optional capability contracts", Requires: []*analysis.Analyzer{provider}, Run: func(pass *analysis.Pass) (any, error) {
			for _, diagnostic := range pass.ResultOf[provider].(capabilityFindings)[id] {
				pass.Report(diagnostic)
			}
			return nil, nil
		}}})
	}
	return result
}

func exportCapabilityHelperFacts(pass *analysis.Pass, functions []*ssa.Function) {
	for _, function := range functions {
		object, _ := function.Object().(*types.Func)
		if object == nil || !object.Exported() || function.Signature.Results().Len() != 1 || !types.Identical(function.Signature.Results().At(0).Type(), types.Typ[types.Bool]) {
			continue
		}
		var returned ssa.Value
		valid := true
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				if ret, ok := instruction.(*ssa.Return); ok {
					if len(ret.Results) != 1 || returned != nil {
						valid = false
						continue
					}
					returned = ret.Results[0]
				}
			}
		}
		guard, ok := resolveCapabilityGuard(pass, returned, true, nil, 0)
		parameter, parameterOK := capabilityIdentity(guard.source).(*ssa.Parameter)
		if !valid || !ok || !parameterOK {
			continue
		}
		index := -1
		for i, candidate := range function.Params {
			if candidate == parameter {
				index = i
				break
			}
		}
		named, _ := guard.target.(*types.Named)
		if named == nil || named.Obj().Pkg() == nil || index < 0 {
			continue
		}
		pass.ExportObjectFact(object, &capabilityHelperFact{Parameter: index, Package: named.Obj().Pkg().Path(), Name: named.Obj().Name(), Negated: !guard.success})
	}
}

func optionalCapability(capabilities []*types.Interface, target types.Type) bool {
	iface, ok := target.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for _, capability := range capabilities {
		if types.Implements(iface, capability) {
			return true
		}
	}
	return false
}

func loadedOptionalCapabilities(pass *analysis.Pass) []*types.Interface {
	// Wrappers may re-export a capability without importing playdate directly
	// in the analyzed package. Resolve the actual loaded SDK type identities.
	queue := []*types.Package{pass.Pkg}
	seen := make(map[*types.Package]bool)
	var sdk *types.Package
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		if pkg.Path() == playdatePackage {
			sdk = pkg
			break
		}
		queue = append(queue, pkg.Imports()...)
	}
	if sdk == nil {
		return nil
	}
	var capabilities []*types.Interface
	for _, contract := range ContractInventory().Contracts {
		if contract.ID != "context-optional-capability" {
			continue
		}
		for _, symbol := range contract.PublicAPI {
			if symbol.Name == "Context" {
				continue
			}
			if object := sdk.Scope().Lookup(symbol.Name); object != nil {
				if capability, ok := object.Type().Underlying().(*types.Interface); ok {
					capabilities = append(capabilities, capability)
				}
			}
		}
	}
	return capabilities
}

func capabilityInStaticInterface(source, target types.Type) bool {
	sourceInterface, ok := source.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	targetInterface, ok := target.Underlying().(*types.Interface)
	return ok && types.Implements(sourceInterface, targetInterface)
}

type capabilityGuard struct {
	source  ssa.Value
	target  types.Type
	success bool
}

func capabilityAt(pass *analysis.Pass, assertion *ssa.TypeAssert) (bool, bool) {
	source := capabilityIdentity(assertion.X)
	if known, present := capabilityOnPathWithPass(pass, source, assertion.AssertedType, assertion.Block()); known {
		return known, present
	}
	return capturedCapability(assertion, source)
}

func capabilityOnPath(source ssa.Value, target types.Type, location *ssa.BasicBlock) (bool, bool) {
	return capabilityOnPathWithPass(nil, source, target, location)
}

func capabilityOnPathWithPass(pass *analysis.Pass, source ssa.Value, target types.Type, location *ssa.BasicBlock) (bool, bool) {
	if nilSSA(source) {
		return true, false
	}
	if boxed, ok := source.(*ssa.MakeInterface); ok {
		if target, ok := target.Underlying().(*types.Interface); ok {
			return true, types.Implements(boxed.X.Type(), target)
		}
	}
	for block := location.Idom(); block != nil; block = block.Idom() {
		if len(block.Instrs) == 0 {
			continue
		}
		branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
		if !ok {
			continue
		}
		truth := true
		if !block.Succs[0].Dominates(location) {
			truth = false
			if !block.Succs[1].Dominates(location) {
				continue
			}
		}
		guard, ok := resolveCapabilityGuard(pass, branch.Cond, truth, nil, 0)
		if !ok || capabilityIdentity(guard.source) != source {
			continue
		}
		if guard.success && capabilityInStaticInterface(guard.target, target) {
			return true, true
		}
		if !guard.success && capabilityInStaticInterface(target, guard.target) {
			return true, false
		}
	}
	return false, false
}

func capturedCapability(assertion *ssa.TypeAssert, source ssa.Value) (bool, bool) {
	function := assertion.Parent()
	parent := function.Parent()
	if parent == nil {
		return false, false
	}
	load, ok := source.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return false, false
	}
	free, ok := load.X.(*ssa.FreeVar)
	if !ok {
		return false, false
	}
	index := -1
	for i, value := range function.FreeVars {
		if value == free {
			index = i
			break
		}
	}
	if index < 0 {
		return false, false
	}
	for _, block := range parent.Blocks {
		for _, instruction := range block.Instrs {
			closure, ok := instruction.(*ssa.MakeClosure)
			if !ok || closure.Fn != function || index >= len(closure.Bindings) {
				continue
			}
			binding := closure.Bindings[index]
			if allocation, ok := binding.(*ssa.Alloc); ok && immutableCapabilityCell(allocation) == nil {
				return false, false
			}
			var initial ssa.Value
			if binding.Referrers() == nil {
				return false, false
			}
			for _, use := range *binding.Referrers() {
				switch use := use.(type) {
				case *ssa.Store:
					if use.Addr != binding || initial != nil {
						return false, false
					}
					initial = use.Val
				case *ssa.UnOp:
					if use.Op != token.MUL {
						return false, false
					}
				case *ssa.DebugRef:
				case *ssa.MakeClosure:
					callee, ok := use.Fn.(*ssa.Function)
					if !ok {
						return false, false
					}
					for i, capture := range use.Bindings {
						if capture == binding && (i >= len(callee.FreeVars) || !readOnlyCapabilityCapture(callee.FreeVars[i])) {
							return false, false
						}
					}
				default:
					return false, false
				}
			}
			if initial == nil {
				return false, false
			}
			// The outer guard commonly reads the captured cell. Each such read is
			// equivalent while its sole initialization remains the only write.
			for _, use := range *binding.Referrers() {
				if outer, ok := use.(*ssa.UnOp); ok {
					if known, present := capabilityOnPath(outer, assertion.AssertedType, block); known {
						return known, present
					}
				}
			}
			return capabilityOnPath(capabilityIdentity(initial), assertion.AssertedType, block)
		}
	}
	return false, false
}

func readOnlyCapabilityCapture(value *ssa.FreeVar) bool {
	if value.Referrers() == nil {
		return true
	}
	for _, use := range *value.Referrers() {
		switch use := use.(type) {
		case *ssa.UnOp:
			if use.Op != token.MUL {
				return false
			}
		case *ssa.DebugRef:
		default:
			return false
		}
	}
	return true
}

func capabilityIdentity(value ssa.Value) ssa.Value {
	for depth := 0; depth < 8; depth++ {
		switch source := value.(type) {
		case *ssa.ChangeInterface:
			value = source.X
		case *ssa.UnOp:
			if source.Op != token.MUL {
				return value
			}
			allocation, ok := source.X.(*ssa.Alloc)
			if !ok {
				return value
			}
			initial := immutableCapabilityCell(allocation)
			if initial == nil {
				return value
			}
			value = initial
		default:
			return value
		}
	}
	return value
}

func immutableCapabilityCell(cell *ssa.Alloc) ssa.Value {
	if cell.Referrers() == nil {
		return nil
	}
	var initial ssa.Value
	var store *ssa.Store
	for _, use := range *cell.Referrers() {
		switch use := use.(type) {
		case *ssa.Store:
			if use.Addr != cell || initial != nil {
				return nil
			}
			initial = use.Val
			store = use
		case *ssa.UnOp:
			if use.Op != token.MUL {
				return nil
			}
		case *ssa.DebugRef:
		case *ssa.MakeClosure:
			function, ok := use.Fn.(*ssa.Function)
			if !ok {
				return nil
			}
			for index, binding := range use.Bindings {
				if binding == cell && (index >= len(function.FreeVars) || !readOnlyCapabilityCapture(function.FreeVars[index])) {
					return nil
				}
			}
		default:
			return nil
		}
	}
	if store == nil {
		return nil
	}
	for _, use := range *cell.Referrers() {
		if use == store {
			continue
		}
		switch use.(type) {
		case *ssa.UnOp, *ssa.MakeClosure:
			if !capabilityInstructionBefore(store, use) {
				return nil
			}
		}
	}
	return initial
}

func capabilityInstructionBefore(first, second ssa.Instruction) bool {
	if !first.Block().Dominates(second.Block()) {
		return false
	}
	if first.Block() != second.Block() {
		return true
	}
	for _, instruction := range first.Block().Instrs {
		if instruction == first {
			return true
		}
		if instruction == second {
			return false
		}
	}
	return false
}

// Only boolean helpers returning one unchanged assertion result (or its
// negation) carry facts. Multiple returns, unknown calls, and memory loads do
// not establish a guard. SSA identities naturally invalidate reassignment.
func resolveCapabilityGuard(pass *analysis.Pass, value ssa.Value, truth bool, bindings map[ssa.Value]ssa.Value, depth int) (capabilityGuard, bool) {
	if depth > 8 {
		return capabilityGuard{}, false
	}
	if bound := bindings[value]; bound != nil {
		return resolveCapabilityGuard(pass, bound, truth, nil, depth+1)
	}
	switch value := value.(type) {
	case *ssa.BinOp:
		if value.Op != token.EQL && value.Op != token.NEQ {
			return capabilityGuard{}, false
		}
		operand, literal := value.X, value.Y
		if _, ok := literal.(*ssa.Const); !ok {
			operand, literal = literal, operand
		}
		if fixed, ok := literal.(*ssa.Const); ok && fixed.Value != nil && fixed.Value.Kind() == constant.Bool {
			want := constant.BoolVal(fixed.Value)
			if value.Op == token.NEQ {
				want = !want
			}
			if !truth {
				want = !want
			}
			return resolveCapabilityGuard(pass, operand, want, bindings, depth+1)
		}
	case *ssa.UnOp:
		if value.Op == token.NOT {
			return resolveCapabilityGuard(pass, value.X, !truth, bindings, depth+1)
		}
	case *ssa.Extract:
		assertion, ok := value.Tuple.(*ssa.TypeAssert)
		if !ok || value.Index != 1 {
			return capabilityGuard{}, false
		}
		source := capabilityIdentity(assertion.X)
		if bound := bindings[source]; bound != nil {
			source = bound
		}
		return capabilityGuard{source, assertion.AssertedType, truth}, true
	case *ssa.Call:
		call := value.Common()
		callee := call.StaticCallee()
		if callee == nil {
			return capabilityGuard{}, false
		}
		if callee.Pkg != value.Parent().Pkg && pass != nil {
			object, _ := callee.Object().(*types.Func)
			var fact capabilityHelperFact
			if object == nil || !pass.ImportObjectFact(object, &fact) || fact.Parameter < 0 || fact.Parameter >= len(call.Args) {
				return capabilityGuard{}, false
			}
			target := namedTypeFromImports(pass.Pkg, fact.Package, fact.Name)
			if target == nil {
				return capabilityGuard{}, false
			}
			return capabilityGuard{source: capabilityIdentity(call.Args[fact.Parameter]), target: target, success: truth != fact.Negated}, true
		}
		if callee.Pkg != value.Parent().Pkg {
			return capabilityGuard{}, false
		}
		arguments := make(map[ssa.Value]ssa.Value)
		for index, argument := range call.Args {
			if bound := bindings[argument]; bound != nil {
				argument = bound
			}
			if index < len(callee.Params) {
				arguments[callee.Params[index]] = argument
			}
		}
		var returned ssa.Value
		for _, block := range callee.Blocks {
			for _, instruction := range block.Instrs {
				if ret, ok := instruction.(*ssa.Return); ok {
					if len(ret.Results) != 1 || returned != nil {
						return capabilityGuard{}, false
					}
					returned = ret.Results[0]
				}
			}
		}
		if returned != nil {
			return resolveCapabilityGuard(pass, returned, truth, arguments, depth+1)
		}
	}
	return capabilityGuard{}, false
}

func namedTypeFromImports(root *types.Package, path, name string) types.Type {
	queue := []*types.Package{root}
	seen := make(map[*types.Package]bool)
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		if pkg.Path() == path {
			if object := pkg.Scope().Lookup(name); object != nil {
				return object.Type()
			}
			return nil
		}
		queue = append(queue, pkg.Imports()...)
	}
	return nil
}
