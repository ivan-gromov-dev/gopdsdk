package analyzer

import (
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

type terminationResource struct {
	value ssa.Value
	pos   token.Pos
}

// checkTerminationResources explores a bounded, local termination path. Only
// acquisitions whose error is proven nil become live. Unknown calls discard
// ownership knowledge: they may transfer or aggregate-clean the resources.
// Fields already live before callback entry require interprocedural ownership
// facts and are intentionally outside this proof.
func checkTerminationResources(function *ssa.Function, result applicationFindings) {
	if function.Name() != "HandleLifecycle" || len(function.Blocks) == 0 {
		return
	}
	var event ssa.Value
	var terminate constant.Value
	for _, parameter := range function.Params {
		if sdkNamed(parameter.Type(), playdatePackage, "LifecycleEvent") {
			event = parameter
		}
	}
	if event == nil {
		return
	}
	if named, ok := types.Unalias(event.Type()).(*types.Named); ok {
		if value, ok := named.Obj().Pkg().Scope().Lookup("LifecycleTerminate").(*types.Const); ok {
			terminate = value.Val()
		}
	}
	if terminate == nil {
		return
	}
	type state struct {
		block   *ssa.BasicBlock
		live    map[ssa.Value]terminationResource
		pending map[ssa.Value]terminationResource
		visited map[*ssa.BasicBlock]bool
	}
	queue := []state{{function.Blocks[0], make(map[ssa.Value]terminationResource), make(map[ssa.Value]terminationResource), make(map[*ssa.BasicBlock]bool)}}
	for steps := 0; len(queue) > 0 && steps < 256; steps++ {
		current := queue[0]
		queue = queue[1:]
		if current.visited[current.block] {
			continue
		}
		current.visited[current.block] = true
		var branch *ssa.If
		for _, instruction := range current.block.Instrs {
			switch instruction := instruction.(type) {
			case *ssa.Call:
				call := instruction.Common()
				if ownedAcquisition(call) {
					var handle ssa.Value = instruction
					var failure ssa.Value
					if instruction.Referrers() != nil {
						for _, ref := range *instruction.Referrers() {
							if extract, ok := ref.(*ssa.Extract); ok {
								if extract.Index == 0 {
									handle = extract
								}
								if extract.Index == 1 {
									failure = extract
								}
							}
						}
					}
					if handle != nil && failure != nil {
						current.pending[failure] = terminationResource{handle, instruction.Pos()}
					}
				} else if call.IsInvoke() && call.Method.Pkg() != nil && call.Method.Pkg().Path() == playdatePackage && call.Method.Name() == "Close" {
					delete(current.live, call.Value)
					for key, resource := range current.pending {
						if resource.value == call.Value {
							delete(current.pending, key)
						}
					}
				} else {
					clear(current.live)
					clear(current.pending)
				}
			case *ssa.Defer, *ssa.Go:
				// A deferred helper may own aggregate cleanup.
				clear(current.live)
				clear(current.pending)
			case *ssa.MakeInterface, *ssa.ChangeInterface, *ssa.MakeClosure, *ssa.Phi:
				clear(current.live)
				clear(current.pending)
			case *ssa.Store:
				delete(current.live, instruction.Val)
				for key, resource := range current.pending {
					if resource.value == instruction.Val {
						delete(current.pending, key)
					}
				}
			case *ssa.Return:
				for _, value := range instruction.Results {
					delete(current.live, value)
				}
				for _, resource := range current.live {
					result.add("application-termination-resource-leak", resource.pos, "resource acquired successfully on a visible termination path reaches return without Close or ownership transfer")
				}
			case *ssa.If:
				branch = instruction
			}
		}
		for index, successor := range current.block.Succs {
			next := state{successor, cloneTerminationMap(current.live), cloneTerminationMap(current.pending), make(map[*ssa.BasicBlock]bool)}
			for block, visited := range current.visited {
				next.visited[block] = visited
			}
			if branch != nil {
				understood := false
				if comparison, ok := branch.Cond.(*ssa.BinOp); ok {
					left, right := comparison.X, comparison.Y
					if truth := applicationScalar(comparison, map[ssa.Value]constant.Value{event: terminate}); truth != nil && truth.Kind() == constant.Bool {
						understood = true
						if constant.BoolVal(truth) != (index == 0) {
							continue
						}
					}
					if nilSSA(left) {
						left, right = right, left
					}
					if resource, ok := next.pending[left]; ok && nilSSA(right) && (comparison.Op == token.EQL || comparison.Op == token.NEQ) {
						understood = true
						delete(next.pending, left)
						if (comparison.Op == token.EQL) == (index == 0) {
							next.live[resource.value] = resource
						}
					}
				}
				// Do not infer feasibility through arbitrary or correlated guards.
				if !understood {
					continue
				}
			}
			queue = append(queue, next)
		}
	}
}

func nilSSA(value ssa.Value) bool { constant, ok := value.(*ssa.Const); return ok && constant.IsNil() }

func cloneTerminationMap(source map[ssa.Value]terminationResource) map[ssa.Value]terminationResource {
	result := make(map[ssa.Value]terminationResource, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func ownedAcquisition(call *ssa.CallCommon) bool {
	for _, operation := range []struct{ receiver, method string }{
		{"Graphics", "NewBitmap"}, {"Graphics", "LoadBitmap"}, {"FontGraphics", "LoadFont"},
		{"Sprites", "NewSprite"}, {"Audio", "LoadSoundEffect"}, {"Audio", "LoadFilePlayer"},
		{"Videos", "LoadVideo"}, {"FileSystem", "OpenFile"},
		{"CallbackAudio", "NewPCMCallbackSource"},
		{"GeneratorSynthesizers", "NewGeneratorSynth"},
	} {
		if callMethod(call, playdatePackage, operation.receiver, operation.method) {
			return true
		}
	}
	return false
}
