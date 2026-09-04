package analyzer

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// A callback's early exit can exclude an Init fallback state. Only a private
// field with exactly one direct write in Init, and no other writes or address
// escapes, can connect these paths. This is not a general heap-state analysis.
func (flow *capabilityFlow) fallbackExcluded(initializer *ssa.Function, ret *ssa.Return, callback *ssa.Function, location *ssa.BasicBlock) bool {
	for block := location.Idom(); block != nil; block = block.Idom() {
		if len(block.Instrs) == 0 {
			continue
		}
		branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
		if !ok {
			continue
		}
		comparison, ok := branch.Cond.(*ssa.BinOp)
		if !ok {
			continue
		}
		value := comparison.X
		if nilSSA(value) {
			value = comparison.Y
		}
		if !nilComparison(comparison, value) {
			continue
		}
		load, ok := value.(*ssa.UnOp)
		if !ok || load.Op != token.MUL {
			continue
		}
		field, ok := ownedFieldAddress(load.X)
		if !ok || load.X.(*ssa.FieldAddr).X != callback.Params[0] {
			continue
		}
		index := -1
		if block.Succs[0].Dominates(location) {
			index = 0
		} else if block.Succs[1].Dominates(location) {
			index = 1
		}
		if index < 0 {
			continue
		}
		wantNonNil := (comparison.Op == token.NEQ) == (index == 0)
		store := flow.singleInitFieldStore(initializer, field)
		if store == nil || !capabilityInstructionBefore(store, ret) {
			continue
		}
		if wantNonNil && nilSSA(store.Val) {
			return true
		}
		if !wantNonNil && flow.definitelyNonNilError(store.Val, store.Block(), 0) {
			return true
		}
	}
	return false
}

func (flow *capabilityFlow) singleInitFieldStore(initializer *ssa.Function, field applicationField) *ssa.Store {
	var store *ssa.Store
	for _, function := range flow.functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				if write, ok := instruction.(*ssa.Store); ok {
					// Whole-object replacement can reset the field without FieldAddr.
					if pointer, ok := write.Addr.Type().Underlying().(*types.Pointer); ok && types.Identical(pointer.Elem(), field.owner) {
						return nil
					}
				}
				address, ok := instruction.(*ssa.FieldAddr)
				if !ok {
					continue
				}
				candidate, ok := ownedFieldAddress(address)
				if !ok || candidate.index != field.index || !types.Identical(candidate.owner, field.owner) {
					continue
				}
				if address.Referrers() == nil {
					return nil
				}
				for _, use := range *address.Referrers() {
					switch use := use.(type) {
					case *ssa.Store:
						if use.Addr != address || store != nil || function != initializer || address.X != initializer.Params[0] {
							return nil
						}
						store = use
					case *ssa.UnOp:
						if use.Op != token.MUL {
							return nil
						}
					case *ssa.DebugRef:
					default:
						return nil
					}
				}
			}
		}
	}
	return store
}
