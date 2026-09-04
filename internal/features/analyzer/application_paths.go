package analyzer

import (
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// walkApplicationCalls carries scalar argument facts across local calls. Unknown
// branches retain only blocks unavoidable on return, never guessed feasibility.
func walkApplicationCalls(function *ssa.Function, depth int, active map[*ssa.Function]bool, visit func(*ssa.CallCommon)) {
	budget := 4096
	if function != nil && function.Name() == "HandleLifecycle" {
		for _, parameter := range function.Params {
			if !sdkNamed(parameter.Type(), playdatePackage, "LifecycleEvent") {
				continue
			}
			named, ok := types.Unalias(parameter.Type()).(*types.Named)
			if !ok {
				continue
			}
			scope := named.Obj().Pkg().Scope()
			for _, name := range scope.Names() {
				if value, ok := scope.Lookup(name).(*types.Const); ok && types.Identical(value.Type(), parameter.Type()) {
					walkApplicationArguments(function, depth, active, map[ssa.Value]constant.Value{parameter: value.Val()}, &budget, visit)
				}
			}
			return
		}
	}
	walkApplicationArguments(function, depth, active, nil, &budget, visit)
}

func walkApplicationArguments(function *ssa.Function, depth int, active map[*ssa.Function]bool, facts map[ssa.Value]constant.Value, budget *int, visit func(*ssa.CallCommon)) {
	if function == nil || depth > 8 || active[function] || len(function.Blocks) == 0 {
		return
	}
	active[function] = true
	defer delete(active, function)
	queue := []*ssa.BasicBlock{function.Blocks[0]}
	seen := make(map[*ssa.BasicBlock]bool)
	for len(queue) > 0 && *budget > 0 {
		*budget -= 1
		block := queue[0]
		queue = queue[1:]
		if seen[block] {
			continue
		}
		seen[block] = true
		for _, instruction := range block.Instrs {
			var call *ssa.CallCommon
			switch instruction := instruction.(type) {
			case *ssa.Call:
				call = instruction.Common()
			case *ssa.Defer:
				call = instruction.Common()
			}
			if call == nil {
				continue
			}
			visit(call)
			callee := call.StaticCallee()
			if callee == nil || callee.Pkg != function.Pkg {
				continue
			}
			arguments := make(map[ssa.Value]constant.Value)
			for index, argument := range call.Args {
				if index < len(callee.Params) {
					if value := applicationScalar(argument, facts); value != nil {
						arguments[callee.Params[index]] = value
					}
				}
			}
			walkApplicationArguments(callee, depth+1, active, arguments, budget, visit)
		}
		if len(block.Instrs) > 0 {
			if branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If); ok {
				if value := applicationScalar(branch.Cond, facts); value != nil && value.Kind() == constant.Bool {
					index := 1
					if constant.BoolVal(value) {
						index = 0
					}
					queue = append(queue, block.Succs[index])
					continue
				}
				// An unknown condition can make either branch impossible for this
				// call. The common continuation is safe to analyze structurally.
				for _, candidate := range function.Blocks {
					if candidate == block || seen[candidate] || !block.Dominates(candidate) {
						continue
					}
					unavoidable := true
					for _, exit := range function.Blocks {
						if len(exit.Succs) == 0 && !candidate.Dominates(exit) {
							unavoidable = false
							break
						}
					}
					if unavoidable {
						queue = append(queue, candidate)
					}
				}
				continue
			}
		}
		queue = append(queue, block.Succs...)
	}
}

func applicationScalar(value ssa.Value, facts map[ssa.Value]constant.Value) constant.Value {
	if known := facts[value]; known != nil {
		return known
	}
	switch value := value.(type) {
	case *ssa.Const:
		return value.Value
	case *ssa.UnOp:
		if value.Op == token.NOT {
			if operand := applicationScalar(value.X, facts); operand != nil && operand.Kind() == constant.Bool {
				return constant.UnaryOp(token.NOT, operand, 0)
			}
		}
	case *ssa.BinOp:
		left, right := applicationScalar(value.X, facts), applicationScalar(value.Y, facts)
		if left == nil || right == nil {
			return nil
		}
		switch value.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return constant.MakeBool(constant.Compare(left, value.Op, right))
		}
	}
	return nil
}
