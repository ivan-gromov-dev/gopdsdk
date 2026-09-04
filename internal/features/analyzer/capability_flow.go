package analyzer

import (
	"go/ast"

	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

type capabilityFlow struct {
	functions []*ssa.Function
	calls     map[*ssa.Function][]capabilityCallSite
	escaped   map[*types.Func]bool
	games     map[types.Type]bool
}

type capabilityCallSite struct {
	call  *ssa.CallCommon
	block *ssa.BasicBlock
}

func (flow *capabilityFlow) globalUses(global *ssa.Global) []ssa.Instruction {
	functions := append([]*ssa.Function(nil), flow.functions...)
	if global.Pkg != nil {
		if initializer := global.Pkg.Func("init"); initializer != nil {
			functions = append(functions, initializer)
		}
	}
	var uses []ssa.Instruction
	for _, function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				for _, operand := range instruction.Operands(nil) {
					if operand != nil && *operand == global {
						uses = append(uses, instruction)
						break
					}
				}
			}
		}
	}
	return uses
}

func newCapabilityFlow(pass *analysis.Pass, functions []*ssa.Function) *capabilityFlow {
	flow := &capabilityFlow{functions: functions, calls: make(map[*ssa.Function][]capabilityCallSite), escaped: make(map[*types.Func]bool), games: make(map[types.Type]bool)}
	direct := make(map[*ast.Ident]bool)
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			expression := ast.Unparen(call.Fun)
			switch indexed := expression.(type) {
			case *ast.IndexExpr:
				expression = ast.Unparen(indexed.X)
			case *ast.IndexListExpr:
				expression = ast.Unparen(indexed.X)
			}
			switch expression := expression.(type) {
			case *ast.Ident:
				direct[expression] = true
			case *ast.SelectorExpr:
				direct[expression.Sel] = true
			}
			return true
		})
	}
	for identifier, object := range pass.TypesInfo.Uses {
		if function, ok := object.(*types.Func); ok && function.Pkg() == pass.Pkg && !direct[identifier] {
			flow.escaped[function] = true
		}
	}
	for _, function := range functions {
		if function.Name() == "New" && function.Signature.Recv() == nil && function.Signature.Params().Len() == 0 && function.Signature.Results().Len() == 1 && sdkNamed(function.Signature.Results().At(0).Type(), playdatePackage, "Game") {
			visitFactoryReturns(function, 0, make(map[*ssa.Function]bool), func(value ssa.Value) {
				if boxed, ok := value.(*ssa.MakeInterface); ok && sdkNamed(boxed.Type(), playdatePackage, "Game") {
					flow.games[boxed.X.Type()] = true
				}
			})
		}
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := call.Common()
				if callee := common.StaticCallee(); callee != nil {
					flow.calls[callee] = append(flow.calls[callee], capabilityCallSite{common, block})
				}
				if common.IsInvoke() {
					for _, candidate := range functions {
						if candidate.Name() == common.Method.Name() {
							if object, ok := candidate.Object().(*types.Func); ok {
								flow.escaped[object] = true
							}
						}
					}
				}
			}
		}
	}
	return flow
}

type capabilityQuery struct {
	parameter *ssa.Parameter
	target    types.Type
}

func (flow *capabilityFlow) present(source ssa.Value, target types.Type, block *ssa.BasicBlock, active map[capabilityQuery]bool, depth int) bool {
	if depth > 8 {
		return false
	}
	source = capabilityIdentity(source)
	if known, present := capabilityOnPath(source, target, block); known {
		return present
	}
	if capabilityInStaticInterface(source.Type(), target) {
		return true
	}
	parameter, ok := source.(*ssa.Parameter)
	if !ok {
		return false
	}
	query := capabilityQuery{parameter, target}
	if active[query] {
		return false
	}
	active[query] = true
	defer delete(active, query)
	function := parameter.Parent()
	object, _ := function.Object().(*types.Func)
	if object == nil || flow.escaped[object] {
		return false
	}
	index := -1
	for i, value := range function.Params {
		if value == parameter {
			index = i
			break
		}
	}
	if index < 0 {
		return false
	}
	if flow.runtimeRequirement(function, parameter, target, block) {
		return true
	}
	// Exported helpers have callers outside this package. Function values and
	// interface dispatch similarly prevent a closed set of call-site proofs.
	if object.Exported() || len(flow.calls[function]) == 0 {
		return false
	}
	for _, site := range flow.calls[function] {
		call := site.call
		if index >= len(call.Args) || !flow.present(call.Args[index], target, site.block, active, depth+1) {
			return false
		}
	}
	return true
}

func (flow *capabilityFlow) runtimeRequirement(function *ssa.Function, parameter *ssa.Parameter, target types.Type, location *ssa.BasicBlock) bool {
	if function.Name() != "Update" && function.Name() != "HandleLifecycle" {
		return false
	}
	if function.Signature.Recv() == nil || !sdkNamed(parameter.Type(), playdatePackage, "Context") {
		return false
	}
	returned := false
	for game := range flow.games {
		if types.Identical(game, function.Signature.Recv().Type()) {
			returned = true
			break
		}
	}
	if !returned {
		return false
	}
	// Explicit manual callback calls can supply a different context.
	if len(flow.calls[function]) != 0 {
		return false
	}
	for _, candidate := range flow.functions {
		if candidate.Name() == "Init" && candidate.Signature.Recv() != nil && types.Identical(candidate.Signature.Recv().Type(), function.Signature.Recv().Type()) {
			for _, context := range candidate.Params {
				if sdkNamed(context.Type(), playdatePackage, "Context") {
					return flow.initRequiresCapability(candidate, context, target, function, location)
				}
			}
		}
	}
	return false
}

// Force the queried capability absent. A requirement is proven only if every
// reachable normal return is a definitely non-nil error. Unknown returns,
// cycles, or exhaustion of the traversal budget end the proof.
func (flow *capabilityFlow) initRequiresCapability(function *ssa.Function, context ssa.Value, target types.Type, callback *ssa.Function, location *ssa.BasicBlock) bool {
	if len(function.Blocks) == 0 {
		return false
	}

	facts := make(map[ssa.Value]constant.Value)
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			if extract, ok := instruction.(*ssa.Extract); ok && extract.Index == 1 {
				if assertion, ok := extract.Tuple.(*ssa.TypeAssert); ok && capabilityIdentity(assertion.X) == context && capabilityInStaticInterface(assertion.AssertedType, target) {
					facts[extract] = constant.MakeBool(false)
				}
			}
		}
	}
	if len(facts) == 0 {
		return false
	}
	active := make(map[*ssa.BasicBlock]bool)
	done := make(map[*ssa.BasicBlock]bool)
	budget := 256
	var visit func(*ssa.BasicBlock) bool
	visit = func(block *ssa.BasicBlock) bool {
		if done[block] {
			return true
		}
		if active[block] || budget == 0 {
			return false
		}
		budget--
		active[block] = true
		defer delete(active, block)
		for _, instruction := range block.Instrs {
			if ret, ok := instruction.(*ssa.Return); ok {
				if len(ret.Results) != 1 || (!flow.definitelyNonNilError(ret.Results[0], block, 0) && !flow.fallbackExcluded(function, ret, callback, location)) {
					return false
				}
			}
		}
		if len(block.Instrs) > 0 {
			if branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If); ok {
				if truth := applicationScalar(branch.Cond, facts); truth != nil && truth.Kind() == constant.Bool {
					index := 1
					if constant.BoolVal(truth) {
						index = 0
					}
					if !visit(block.Succs[index]) {
						return false
					}
					done[block] = true
					return true
				}
			}
		}
		for _, successor := range block.Succs {
			if !visit(successor) {
				return false
			}
		}
		done[block] = true
		return true
	}
	return visit(function.Blocks[0])
}

func (flow *capabilityFlow) definitelyNonNilError(value ssa.Value, location *ssa.BasicBlock, depth int) bool {
	if depth > 8 || nilSSA(value) {
		return false
	}
	if _, ok := value.(*ssa.MakeInterface); ok {
		return true
	}
	for block := location.Idom(); block != nil; block = block.Idom() {
		if len(block.Instrs) == 0 {
			continue
		}
		branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
		if !ok {
			continue
		}
		comparison, ok := branch.Cond.(*ssa.BinOp)
		if !ok || !nilComparison(comparison, value) {
			continue
		}
		index := 0
		if comparison.Op == token.EQL {
			index = 1
		}
		if block.Succs[index].Dominates(location) {
			return true
		}
	}
	if call, ok := value.(*ssa.Call); ok {
		callee := call.Common().StaticCallee()
		if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg.Path() != "errors" {
			return false
		}
		if callee.Name() == "New" {
			return true
		}
	}
	if load, ok := value.(*ssa.UnOp); ok && load.Op == token.MUL {
		global, ok := load.X.(*ssa.Global)
		if !ok || global.Object() == nil || global.Object().Exported() {
			return false
		}
		var initial *ssa.Store
		for _, use := range flow.globalUses(global) {
			switch use := use.(type) {
			case *ssa.Store:
				if use.Addr != global || initial != nil {
					return false
				}
				initial = use
			case *ssa.UnOp:
				if use.Op != token.MUL {
					return false
				}
			case *ssa.DebugRef:
			default:
				return false
			}
		}
		return initial != nil && flow.definitelyNonNilError(initial.Val, initial.Block(), depth+1)
	}
	return false
}
