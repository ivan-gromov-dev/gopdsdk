package analyzer

import (
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

type applicationField struct {
	owner *types.Named
	index int
}

func ownedFieldAddress(value ssa.Value) (applicationField, bool) {
	address, ok := value.(*ssa.FieldAddr)
	if !ok {
		return applicationField{}, false
	}
	pointer, ok := address.X.Type().Underlying().(*types.Pointer)
	if !ok {
		return applicationField{}, false
	}
	owner, ok := types.Unalias(pointer.Elem()).(*types.Named)
	if !ok {
		return applicationField{}, false
	}
	structure, ok := owner.Underlying().(*types.Struct)
	if !ok || structure.Field(address.Field).Exported() {
		return applicationField{}, false
	}
	return applicationField{owner, address.Field}, true
}

// Field ownership is inferred only for private fields whose every visible
// non-nil write comes directly from an owned constructor. Any handle alias,
// method call, aggregate transfer, or address escape invalidates that proof.
// This intentionally gives cleanup helpers and native aggregate cleanup the
// benefit of the doubt instead of reporting their retained children as leaks.
func checkApplicationFields(pass *analysis.Pass, functions []*ssa.Function, result applicationFindings) {
	acquired := make(map[applicationField]bool)
	unknown := make(map[applicationField]bool)
	for _, function := range functions {
		if PassContext(pass).Err() != nil {
			return
		}
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				address, ok := instruction.(*ssa.FieldAddr)
				if !ok {
					continue
				}
				field, ok := ownedFieldAddress(address)
				if !ok {
					continue
				}
				if address.Referrers() == nil {
					continue
				}
				for _, use := range *address.Referrers() {
					switch use := use.(type) {
					case *ssa.Store:
						if use.Addr != address {
							unknown[field] = true
							continue
						}
						if nilSSA(use.Val) {
							continue
						}
						extract, ok := use.Val.(*ssa.Extract)
						if !ok || extract.Index != 0 {
							unknown[field] = true
							continue
						}
						call, ok := extract.Tuple.(*ssa.Call)
						if !ok || !ownedAcquisition(call.Common()) {
							unknown[field] = true
							continue
						}
						acquired[field] = true
						for _, alias := range *extract.Referrers() {
							if alias != use {
								if comparison, ok := alias.(*ssa.BinOp); !ok || !nilComparison(comparison, extract) {
									unknown[field] = true
								}
							}
						}
					case *ssa.UnOp:
						if use.Op != token.MUL || use.Referrers() == nil {
							unknown[field] = true
							continue
						}
						for _, read := range *use.Referrers() {
							if comparison, ok := read.(*ssa.BinOp); !ok || !nilComparison(comparison, use) {
								unknown[field] = true
							}
						}
					case *ssa.DebugRef:
					default:
						unknown[field] = true
					}
				}
			}
		}
	}
	for field := range acquired {
		if unknown[field] {
			delete(acquired, field)
		}
	}
	if len(acquired) == 0 {
		return
	}
	for _, function := range functions {
		if function.Name() != "HandleLifecycle" || function.Signature.Recv() == nil || len(function.Blocks) == 0 {
			continue
		}
		game, lifecycle := sdkInterface(pass, "Game"), sdkInterface(pass, "LifecycleGame")
		if game == nil || lifecycle == nil || !types.Implements(function.Signature.Recv().Type(), game) || !types.Implements(function.Signature.Recv().Type(), lifecycle) {
			continue
		}
		var event ssa.Value
		var termination constant.Value
		for _, parameter := range function.Params {
			if sdkNamed(parameter.Type(), playdatePackage, "LifecycleEvent") {
				event = parameter
				named := types.Unalias(parameter.Type()).(*types.Named)
				if object, ok := named.Obj().Pkg().Scope().Lookup("LifecycleTerminate").(*types.Const); ok {
					termination = object.Val()
				}
			}
		}
		if termination == nil {
			continue
		}
		walkFieldTermination(function, event, termination, acquired, result)
	}
}

func nilComparison(comparison *ssa.BinOp, value ssa.Value) bool {
	return (comparison.Op == token.EQL || comparison.Op == token.NEQ) && ((comparison.X == value && nilSSA(comparison.Y)) || (comparison.Y == value && nilSSA(comparison.X)))
}

func walkFieldTermination(function *ssa.Function, event ssa.Value, termination constant.Value, owned map[applicationField]bool, result applicationFindings) {
	type path struct {
		block   *ssa.BasicBlock
		live    map[applicationField]bool
		visited map[*ssa.BasicBlock]bool
	}
	queue := []path{{function.Blocks[0], make(map[applicationField]bool), make(map[*ssa.BasicBlock]bool)}}
	for budget := 0; len(queue) > 0 && budget < 256; budget++ {
		current := queue[0]
		queue = queue[1:]
		if current.visited[current.block] {
			continue
		}
		current.visited[current.block] = true
		valid := true
		for _, instruction := range current.block.Instrs {
			switch instruction := instruction.(type) {
			case *ssa.Call, *ssa.Defer, *ssa.Go:
				valid = false
			case *ssa.Store:
				if field, ok := ownedFieldAddress(instruction.Addr); ok && instruction.Addr.(*ssa.FieldAddr).X == function.Params[0] && current.live[field] && nilSSA(instruction.Val) {
					result.add("application-termination-resource-leak", instruction.Pos(), "termination discards a non-nil owned field without cleanup or ownership transfer")
					current.live[field] = false
				}
			case *ssa.Return:
				if valid {
					for _, live := range current.live {
						if live {
							result.add("application-termination-resource-leak", instruction.Pos(), "termination returns with a non-nil owned field and no cleanup or ownership transfer")
							break
						}
					}
				}
			}
			if !valid {
				break
			}
		}
		if !valid {
			continue
		}
		for index, successor := range current.block.Succs {
			next := path{successor, make(map[applicationField]bool), make(map[*ssa.BasicBlock]bool)}
			for field, live := range current.live {
				next.live[field] = live
			}
			for block, seen := range current.visited {
				next.visited[block] = seen
			}
			if branch, ok := current.block.Instrs[len(current.block.Instrs)-1].(*ssa.If); ok {
				if truth := applicationScalar(branch.Cond, map[ssa.Value]constant.Value{event: termination}); truth != nil && truth.Kind() == constant.Bool {
					if constant.BoolVal(truth) != (index == 0) {
						continue
					}
				} else {
					comparison, ok := branch.Cond.(*ssa.BinOp)
					if !ok {
						continue
					}
					value := comparison.X
					if nilSSA(value) {
						value = comparison.Y
					}
					load, ok := value.(*ssa.UnOp)
					if !ok || !nilComparison(comparison, value) {
						continue
					}
					field, ok := ownedFieldAddress(load.X)
					if !ok || !owned[field] {
						continue
					}
					// Only the active game's receiver, not another object of the
					// same type, can establish this callback's field state.
					if load.X.(*ssa.FieldAddr).X != function.Params[0] {
						continue
					}
					live := (comparison.Op == token.NEQ) == (index == 0)
					if old, known := current.live[field]; known && old != live {
						continue
					}
					next.live[field] = live
				}
			}
			queue = append(queue, next)
		}
	}
}
