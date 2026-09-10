package analyzer

import (
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

const (
	ownershipEffectClose uint8 = 1 << iota
	ownershipEffectTransfer
)

const (
	ownershipResultUnknown uint8 = iota
	ownershipResultOwned
	ownershipResultBorrowed
)

type ownershipSummaryFact struct {
	Effects        []uint8
	Results        []uint8
	Retained       []int
	RetentionKinds []string
	Path           string
}

func (*ownershipSummaryFact) AFact() {}

type ownershipSummaries map[*ssa.Function]ownershipSummaryFact

func buildOwnershipSummaries(pass *analysis.Pass, functions []*ssa.Function) (ownershipSummaries, bool) {
	result := make(ownershipSummaries)
	exhausted := false
	for round := 0; round < 8; round++ {
		changed := false
		for _, function := range functions {
			next := summarizeOwnershipFunction(pass, function, result)
			if !sameOwnershipSummary(result[function], next) {
				result[function], changed = next, true
			}
		}
		if !changed {
			exhausted = false
			goto converged
		}
		exhausted = true
	}
converged:
	for function, summary := range result {
		object, _ := function.Object().(*types.Func)
		if object != nil && object.Exported() && (hasOwnershipSummary(summary)) {
			copy := summary
			pass.ExportObjectFact(object, &copy)
		}
	}
	return result, exhausted
}

func summarizeOwnershipFunction(pass *analysis.Pass, function *ssa.Function, known ownershipSummaries) ownershipSummaryFact {
	summary := ownershipSummaryFact{Effects: make([]uint8, len(function.Params)), Results: make([]uint8, function.Signature.Results().Len()), Path: function.String()}
	parameter := make(map[ssa.Value]int)
	for i, value := range function.Params {
		parameter[value] = i
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			if value, ok := instruction.(ssa.Value); ok {
				if index, ok := summaryAliasParameter(value, parameter); ok {
					parameter[value] = index
				}
			}
			switch instruction := instruction.(type) {
			case *ssa.Call:
				common := instruction.Common()
				path, receiver, name, _ := sdkCall(common)
				if name == "Close" && (common.IsInvoke() || len(common.Args) != 0) {
					receiverValue := common.Value
					if !common.IsInvoke() {
						receiverValue = common.Args[0]
					}
					if index, ok := parameter[receiverValue]; ok {
						summary.Effects[index] |= ownershipEffectClose
					}
				} else if stringsHasSDKPrefix(path) {
					if argument, kind, ok := ownershipRetentionArgument(common, receiver+"."+name); ok {
						if index, exists := parameter[argument]; exists {
							summary.Retained = appendUniqueRetention(summary.Retained, summary.RetentionKinds, index, kind, &summary.RetentionKinds)
						}
					}
				} else if path == "" || !stringsHasSDKPrefix(path) {
					if called, ok := ownershipCallSummary(pass, common, known); ok {
						for argument, effect := range called.Effects {
							if value, exists := ownershipCallArgument(common, argument); exists {
								if index, ok := parameter[value]; ok {
									summary.Effects[index] |= effect
								}
							}
						}
						for retainedIndex, retained := range called.Retained {
							if value, exists := ownershipCallArgument(common, retained); exists {
								if index, ok := parameter[value]; ok {
									kind := "helper retention"
									if retainedIndex < len(called.RetentionKinds) {
										kind = called.RetentionKinds[retainedIndex]
									}
									summary.Retained = appendUniqueRetention(summary.Retained, summary.RetentionKinds, index, kind, &summary.RetentionKinds)
								}
							}
						}
					} else if common.StaticCallee() == nil || common.StaticCallee().Pkg != function.Pkg {
						for _, argument := range common.Args {
							if index, ok := parameter[argument]; ok {
								summary.Effects[index] |= ownershipEffectTransfer
							}
						}
					}
				}
			case *ssa.Return:
				for index, value := range instruction.Results {
					if parameterIndex, ok := parameter[value]; ok {
						summary.Effects[parameterIndex] |= ownershipEffectTransfer
						continue
					}
					if extract, ok := value.(*ssa.Extract); ok {
						if kind := ownershipResultForCall(pass, extract, known); kind != 0 {
							summary.Results[index] = kind
						}
					}
				}
			}
		}
	}
	return summary
}

func stringsHasSDKPrefix(path string) bool {
	return len(path) >= len(sdkPackagePrefix) && path[:len(sdkPackagePrefix)] == sdkPackagePrefix
}

func summaryAliasParameter(value ssa.Value, parameters map[ssa.Value]int) (int, bool) {
	var inputs []ssa.Value
	switch value := value.(type) {
	case *ssa.ChangeInterface:
		inputs = []ssa.Value{value.X}
	case *ssa.ChangeType:
		inputs = []ssa.Value{value.X}
	case *ssa.Convert:
		inputs = []ssa.Value{value.X}
	case *ssa.MakeInterface:
		inputs = []ssa.Value{value.X}
	case *ssa.Phi:
		inputs = value.Edges
	default:
		return 0, false
	}
	index := -1
	for _, input := range inputs {
		candidate, ok := parameters[input]
		if !ok || index >= 0 && candidate != index {
			return 0, false
		}
		index = candidate
	}
	return index, index >= 0
}

func ownershipResultForCall(pass *analysis.Pass, extract *ssa.Extract, known ownershipSummaries) uint8 {
	call, ok := extract.Tuple.(*ssa.Call)
	if !ok {
		return 0
	}
	path, receiver, name, _ := sdkCall(call.Common())
	if stringsHasSDKPrefix(path) {
		if extract.Index == 0 && ownedConstructor(receiver+"."+name) {
			return ownershipResultOwned
		}
		if extract.Index == 0 && isBorrowedHandleConstructor(receiver+"."+name) {
			return ownershipResultBorrowed
		}
		return 0
	}
	if summary, ok := ownershipCallSummary(pass, call.Common(), known); ok && extract.Index < len(summary.Results) {
		return summary.Results[extract.Index]
	}
	return 0
}

func ownershipCallSummary(pass *analysis.Pass, common *ssa.CallCommon, local ownershipSummaries) (ownershipSummaryFact, bool) {
	callee := common.StaticCallee()
	if callee == nil && common.IsInvoke() {
		var selected ownershipSummaryFact
		count := 0
		for function, summary := range local {
			if function.Name() != common.Method.Name() || !hasOwnershipSummary(summary) || function.Signature.Recv() == nil {
				continue
			}
			iface, _ := common.Value.Type().Underlying().(*types.Interface)
			if iface == nil || !types.Implements(function.Signature.Recv().Type(), iface) {
				continue
			}
			count++
			if count > 4 || count > 1 && !sameOwnershipSummaryEffects(selected, summary) {
				return ownershipSummaryFact{}, false
			}
			selected = summary
		}
		return selected, count != 0
	}
	if callee == nil {
		return ownershipSummaryFact{}, false
	}
	if summary, ok := local[callee]; ok && hasOwnershipSummary(summary) {
		return summary, true
	}
	object, _ := callee.Object().(*types.Func)
	if object == nil || object.Pkg() == pass.Pkg {
		return ownershipSummaryFact{}, false
	}
	var fact ownershipSummaryFact
	return fact, pass.ImportObjectFact(object, &fact)
}

func applyOwnershipSummary(call *ssa.Call, summary ownershipSummaryFact, values map[ssa.Value]handleInfo, state *ownershipState) {
	for index, effect := range summary.Effects {
		argument, exists := ownershipCallArgument(call.Common(), index)
		if !exists {
			continue
		}
		info, ok := values[argument]
		if !ok {
			continue
		}
		current := state.handles[info.id]
		if effect&ownershipEffectClose != 0 {
			current.closed = true
			current.closedAt = call.Pos()
			current.closedBy = "handle is closed through " + summary.Path
		}
		if effect&ownershipEffectTransfer != 0 {
			current.escaped = true
		}
		state.handles[info.id] = current
	}
	for index, parameter := range summary.Retained {
		argument, exists := ownershipCallArgument(call.Common(), parameter)
		if !exists {
			continue
		}
		info, ok := values[argument]
		if !ok {
			continue
		}
		kind := "helper retention"
		if index < len(summary.RetentionKinds) {
			kind = summary.RetentionKinds[index]
		}
		state.edges[info.id] = retentionEdge{pos: call.Pos(), kind: kind}
		current := state.handles[info.id]
		current.escaped = true
		state.handles[info.id] = current
	}
}

func ownershipRetentionArgument(common *ssa.CallCommon, key string) (ssa.Value, string, bool) {
	index, kind := -1, ""
	switch key {
	case "SystemControls.SetMenuImage":
		index, kind = 0, "menu image"
	case "Sprite.SetBitmap":
		index, kind = 0, "sprite image"
	case "Sprite.SetStencilImage":
		index, kind = 0, "sprite stencil"
	case "Sprite.SetTileMap":
		index, kind = 0, "sprite tilemap"
	case "VideoPlayer.SetContext":
		index, kind = 0, "video context"
	case "SamplePlayerControls.SetSample":
		index, kind = 0, "player sample"
	case "AudioChannel.AddSource":
		index, kind = 0, "audio channel source"
	case "AudioChannel.AddEffect":
		index, kind = 0, "audio channel effect"
	case "Instrument.AddVoice":
		index, kind = 0, "instrument voice"
	case "SequenceTrack.SetInstrument":
		index, kind = 0, "track instrument"
	case "Sequence.SetTrack":
		index, kind = 1, "sequence track"
	}
	if index < 0 {
		return nil, "", false
	}
	if !common.IsInvoke() {
		index++
	}
	if index >= len(common.Args) {
		return nil, "", false
	}
	return common.Args[index], kind, true
}

func appendUniqueRetention(indices []int, kinds []string, index int, kind string, kindDestination *[]string) []int {
	for i, existing := range indices {
		if existing == index && i < len(kinds) && kinds[i] == kind {
			return indices
		}
	}
	*kindDestination = append(*kindDestination, kind)
	return append(indices, index)
}

func ownershipCallArgument(common *ssa.CallCommon, index int) (ssa.Value, bool) {
	if common.IsInvoke() {
		if index == 0 {
			return common.Value, true
		}
		index--
	}
	if index < 0 || index >= len(common.Args) {
		return nil, false
	}
	return common.Args[index], true
}

func sameOwnershipSummaryEffects(left, right ownershipSummaryFact) bool {
	left.Path, right.Path = "", ""
	return sameOwnershipSummary(left, right)
}

func hasOwnershipSummary(summary ownershipSummaryFact) bool {
	for _, value := range summary.Effects {
		if value != 0 {
			return true
		}
	}
	for _, value := range summary.Results {
		if value != 0 {
			return true
		}
	}
	return len(summary.Retained) != 0
}
func sameOwnershipSummary(left, right ownershipSummaryFact) bool {
	if left.Path != right.Path || len(left.Effects) != len(right.Effects) || len(left.Results) != len(right.Results) {
		return false
	}
	if len(left.Retained) != len(right.Retained) || len(left.RetentionKinds) != len(right.RetentionKinds) {
		return false
	}
	for i := range left.Retained {
		if left.Retained[i] != right.Retained[i] || left.RetentionKinds[i] != right.RetentionKinds[i] {
			return false
		}
	}
	for i := range left.Effects {
		if left.Effects[i] != right.Effects[i] {
			return false
		}
	}
	for i := range left.Results {
		if left.Results[i] != right.Results[i] {
			return false
		}
	}
	return true
}
