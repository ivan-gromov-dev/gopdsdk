package analyzer

import (
	"fmt"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"go/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type ownershipFindings map[RuleID][]analysis.Diagnostic

type handleKind uint8

const (
	handleOwned handleKind = iota + 1
	handleBorrowed
	handleWrapper
)

type handleInfo struct {
	id, parent int
	kind       handleKind
	typeName   string
	created    token.Pos
	origin     ssa.Value
}

type handleState struct {
	closed   bool
	closedAt token.Pos
	closedBy string
	deferred bool
	escaped  bool
	proven   bool
}

type retentionEdge struct {
	owner int
	pos   token.Pos
	kind  string
}

type ownershipState struct {
	handles map[int]handleState
	edges   map[int]retentionEdge // retained handle -> retainer
}

func ownershipRuleRegistrations() []Registration {
	provider := &analysis.Analyzer{Name: "sdkownership", Doc: "track local and bounded interprocedural SDK handle ownership and retention", Requires: []*analysis.Analyzer{buildssa.Analyzer}, ResultType: reflect.TypeOf(ownershipFindings{}), FactTypes: []analysis.Fact{new(ownershipSummaryFact)}, Run: func(pass *analysis.Pass) (any, error) {
		findings := make(ownershipFindings)
		functions := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA).SrcFuncs
		summaries, exhausted := buildOwnershipSummaries(pass, functions)
		if deepAnalysisEnabled(pass) && exhausted && len(functions) != 0 {
			findings.add("ownership-summary-budget", analysis.Diagnostic{Pos: functions[0].Pos(), Message: "interprocedural ownership summary budget was exhausted; results are lower-confidence and unresolved calls remain invalidated"})
		}
		for _, function := range functions {
			checkOwnershipFunction(pass, function, summaries, findings)
		}
		return findings, PassContext(pass).Err()
	}}
	ids := []RuleID{"ownership-resource-leak", "ownership-double-close", "ownership-use-after-close", "ownership-bitmap-use-after-close", "ownership-borrowed-close", "ownership-retained-close", "ownership-close-order", "ownership-summary-budget"}
	result := make([]Registration, 0, len(ids))
	for _, id := range ids {
		result = append(result, Registration{RuleID: id, Analyzer: &analysis.Analyzer{Name: strings.ReplaceAll(string(id), "-", "_"), Doc: "report a proven local ownership violation", Requires: []*analysis.Analyzer{provider}, Run: func(pass *analysis.Pass) (any, error) {
			for _, diagnostic := range pass.ResultOf[provider].(ownershipFindings)[id] {
				pass.Report(diagnostic)
			}
			return nil, nil
		}}})
	}
	return result
}

func (findings ownershipFindings) add(id RuleID, diagnostic analysis.Diagnostic) {
	if !diagnostic.Pos.IsValid() {
		return
	}
	for _, prior := range findings[id] {
		if prior.Pos == diagnostic.Pos && prior.Message == diagnostic.Message {
			return
		}
	}
	findings[id] = append(findings[id], diagnostic)
}

func checkOwnershipFunction(pass *analysis.Pass, function *ssa.Function, summaries ownershipSummaries, findings ownershipFindings) {
	infos, values := discoverHandles(pass, function, summaries)
	if len(infos) == 0 || len(function.Blocks) == 0 {
		return
	}
	initial := ownershipState{handles: make(map[int]handleState), edges: make(map[int]retentionEdge)}
	work := []struct {
		block *ssa.BasicBlock
		state ownershipState
	}{{function.Blocks[0], initial}}
	seen := make(map[string]bool)
	for visits := 0; len(work) != 0 && visits < 256; visits++ {
		item := work[0]
		work = work[1:]
		key := ownershipStateKey(item.block, item.state)
		if seen[key] {
			continue
		}
		seen[key] = true
		state := cloneOwnershipState(item.state)
		for _, instruction := range item.block.Instrs {
			if value, ok := instruction.(ssa.Value); ok {
				if info, exists := values[value]; exists {
					if value == info.origin {
						state.handles[info.id] = handleState{}
					} else if _, initialized := state.handles[info.id]; !initialized {
						state.handles[info.id] = handleState{}
					}
				}
			}
			call, ok := instruction.(*ssa.Call)
			if ok {
				applyOwnershipCall(pass, call, summaries, infos, values, &state, findings)
				continue
			}
			if deferred, ok := instruction.(*ssa.Defer); ok {
				applyDeferredClose(deferred, values, &state, findings)
			}
		}
		if len(item.block.Succs) == 0 {
			reportOwnershipLeaks(function, infos, state, findings)
		}
		for successorIndex, successor := range item.block.Succs {
			nextState := state
			if id, successIndex, ok := acquisitionBranch(item.block, values); ok {
				nextState = cloneOwnershipState(state)
				if successorIndex != successIndex {
					delete(nextState.handles, id)
					delete(nextState.edges, id)
				} else {
					current := nextState.handles[id]
					current.proven = true
					nextState.handles[id] = current
				}
			}
			work = append(work, struct {
				block *ssa.BasicBlock
				state ownershipState
			}{successor, nextState})
		}
	}
}

func applyDeferredClose(deferred *ssa.Defer, values map[ssa.Value]handleInfo, state *ownershipState, findings ownershipFindings) {
	common := deferred.Common()
	path, _, name, _ := sdkCall(common)
	if !strings.HasPrefix(path, sdkPackagePrefix) || name != "Close" {
		return
	}
	receiver := common.Value
	if !common.IsInvoke() && len(common.Args) != 0 {
		receiver = common.Args[0]
	}
	info, ok := values[receiver]
	if !ok {
		return
	}
	if info.kind == handleBorrowed {
		findings.add("ownership-borrowed-close", analysis.Diagnostic{Pos: deferred.Pos(), Message: fmt.Sprintf("borrowed %s cannot be closed by the caller", info.typeName)})
		return
	}
	current := state.handles[info.id]
	current.deferred = true
	state.handles[info.id] = current
}

func acquisitionBranch(block *ssa.BasicBlock, values map[ssa.Value]handleInfo) (int, int, bool) {
	if len(block.Instrs) == 0 || len(block.Succs) != 2 {
		return 0, 0, false
	}
	branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if !ok {
		return 0, 0, false
	}
	comparison, ok := branch.Cond.(*ssa.BinOp)
	if !ok || comparison.Op != token.EQL && comparison.Op != token.NEQ {
		return 0, 0, false
	}
	var extract *ssa.Extract
	if candidate, ok := comparison.X.(*ssa.Extract); ok && candidate.Index > 0 && nilSSAValue(comparison.Y) {
		extract = candidate
	} else if candidate, ok := comparison.Y.(*ssa.Extract); ok && candidate.Index > 0 && nilSSAValue(comparison.X) {
		extract = candidate
	}
	if extract == nil {
		return 0, 0, false
	}
	for value, info := range values {
		owned, ok := value.(*ssa.Extract)
		if ok && owned.Index == 0 && owned.Tuple == extract.Tuple {
			success := 0
			if comparison.Op == token.NEQ {
				success = 1
			}
			return info.id, success, true
		}
	}
	return 0, 0, false
}

func nilSSAValue(value ssa.Value) bool {
	constant, ok := value.(*ssa.Const)
	return ok && constant.IsNil()
}

func discoverHandles(pass *analysis.Pass, function *ssa.Function, summaries ownershipSummaries) (map[int]handleInfo, map[ssa.Value]handleInfo) {
	infos := make(map[int]handleInfo)
	values := make(map[ssa.Value]handleInfo)
	next := 1
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			value, ok := instruction.(ssa.Value)
			if !ok || !isHandleType(value.Type()) {
				continue
			}
			kind, parent, known := classifyHandleValue(pass, value, values, summaries)
			if !known {
				continue
			}
			created := value.Pos()
			if extract, ok := value.(*ssa.Extract); ok {
				created = extract.Tuple.Pos()
			}
			info := handleInfo{id: next, kind: kind, typeName: namedTypeName(value.Type()), created: created, origin: value}
			if parentInfo, exists := values[parent]; exists {
				info.parent = parentInfo.id
			}
			infos[next], values[value] = info, info
			next++
		}
	}
	// Interface and phi aliases retain one identity when all inputs agree.
	for changed := true; changed; {
		changed = false
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				value, ok := instruction.(ssa.Value)
				if !ok || values[value].id != 0 {
					continue
				}
				if info, ok := aliasHandle(value, values); ok {
					values[value] = info
					changed = true
				}
			}
		}
	}
	return infos, values
}

func classifyHandleValue(pass *analysis.Pass, value ssa.Value, known map[ssa.Value]handleInfo, summaries ownershipSummaries) (handleKind, ssa.Value, bool) {
	extract, ok := value.(*ssa.Extract)
	if !ok {
		return 0, nil, false
	}
	call, ok := extract.Tuple.(*ssa.Call)
	if !ok {
		return 0, nil, false
	}
	if deepAnalysisEnabled(pass) {
		if summary, ok := ownershipCallSummary(pass, call.Common(), summaries); ok && extract.Index < len(summary.Results) {
			switch summary.Results[extract.Index] {
			case ownershipResultOwned:
				return handleOwned, nil, true
			case ownershipResultBorrowed:
				return handleBorrowed, nil, true
			}
		}
	}
	path, receiver, name, _ := sdkCall(call.Common())
	if !strings.HasPrefix(path, sdkPackagePrefix) {
		return 0, nil, false
	}
	key := receiver + "." + name
	borrowed := isBorrowedHandleConstructor(key)
	wrapper := map[string]bool{"AudioChannel.DryLevelSignal": true, "AudioChannel.WetLevelSignal": true, "AudioOutputs.DefaultAudioChannel": true}
	if borrowed || wrapper[key] {
		var parent ssa.Value
		if call.Common().IsInvoke() {
			parent = call.Common().Value
		} else if len(call.Common().Args) != 0 {
			parent = call.Common().Args[0]
		}
		if wrapper[key] {
			return handleWrapper, parent, true
		}
		return handleBorrowed, parent, true
	}
	if extract.Index == 0 && ownedConstructor(key) {
		return handleOwned, nil, true
	}
	return 0, nil, false
}

func isBorrowedHandleConstructor(key string) bool {
	return map[string]bool{"BitmapTable.Frame": true, "TextGraphics.Glyph": true, "AudioSample.Data": true, "AudioChannel.Output": true, "Sequence.Track": true, "SequenceTrack.Instrument": true, "SequenceTrack.ControlSignal": true, "SequenceTrack.SignalForController": true}[key]
}

func ownedConstructor(key string) bool {
	constructors := map[string]bool{
		"Graphics.LoadBitmap": true, "Graphics.LoadBitmapTable": true, "Graphics.NewBitmap": true,
		"BitmapDataGraphics.CopyBitmap": true, "BitmapDataGraphics.NewBitmapTable": true, "BitmapDataGraphics.BitmapMask": true, "BitmapDataGraphics.RotatedBitmap": true, "BitmapDataGraphics.CopyDisplayBuffer": true,
		"FontGraphics.LoadFont": true, "FileSystem.OpenFile": true, "Videos.LoadVideo": true, "Sprites.NewSprite": true, "SpriteTileMaps.NewSpriteTileMap": true,
		"Audio.LoadSoundEffect": true, "Audio.LoadFilePlayer": true, "SamplePlayers.LoadSamplePlayer": true, "SamplePlayerFactory.NewSamplePlayer": true, "PCMPlayers.NewPCMPlayer": true,
		"AudioSamples.NewSample": true, "AudioSamples.LoadSample": true, "AudioSamples.NewSampleFromData": true, "CallbackAudio.NewPCMCallbackSource": true, "Microphones.StartMicrophoneRecording": true,
		"AudioChannels.NewAudioChannel": true, "AudioEffects.NewTwoPoleFilter": true, "AudioEffects.NewOnePoleFilter": true, "AudioEffects.NewBitCrusher": true, "AudioEffects.NewRingModulator": true, "AudioEffects.NewDelayLine": true, "AudioEffects.NewOverdrive": true, "DelayLine.AddTap": true,
		"Synthesizers.NewSynth": true, "Synthesizers.NewLFO": true, "Synthesizers.NewEnvelope": true, "Synthesizers.NewControlSignal": true, "GeneratorSynthesizers.NewGeneratorSynth": true,
		"Sequencers.NewInstrument": true, "Sequencers.NewSequenceTrack": true, "Sequencers.NewSequence": true,
	}
	return constructors[key]
}

func isHandleType(typ types.Type) bool {
	method, _, _ := types.LookupFieldOrMethod(typ, true, nil, "Close")
	return method != nil
}

func aliasHandle(value ssa.Value, known map[ssa.Value]handleInfo) (handleInfo, bool) {
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
	case *ssa.TypeAssert:
		inputs = []ssa.Value{value.X}
	case *ssa.Phi:
		inputs = value.Edges
	default:
		return handleInfo{}, false
	}
	var result handleInfo
	for _, input := range inputs {
		info, ok := known[input]
		if !ok || info.id == 0 || result.id != 0 && result.id != info.id {
			return handleInfo{}, false
		}
		result = info
	}
	return result, result.id != 0
}

func applyOwnershipCall(pass *analysis.Pass, call *ssa.Call, summaries ownershipSummaries, infos map[int]handleInfo, values map[ssa.Value]handleInfo, state *ownershipState, findings ownershipFindings) {
	common := call.Common()
	path, receiver, name, _ := sdkCall(common)
	args := common.Args
	var receiverValue ssa.Value
	if common.IsInvoke() {
		receiverValue = common.Value
	} else if receiver != "" && len(args) != 0 {
		receiverValue, args = args[0], args[1:]
	}
	receiverInfo, receiverKnown := values[receiverValue]
	if !strings.HasPrefix(path, sdkPackagePrefix) {
		if deepAnalysisEnabled(pass) {
			if summary, ok := ownershipCallSummary(pass, common, summaries); ok {
				applyOwnershipSummary(call, summary, values, state)
				return
			}
		}
		for _, argument := range common.Args {
			if info, ok := values[argument]; ok {
				value := state.handles[info.id]
				value.escaped = true
				state.handles[info.id] = value
			}
		}
		return
	}
	if receiverKnown {
		current := state.handles[receiverInfo.id]
		if name == "Close" {
			if receiverInfo.kind == handleBorrowed {
				findings.add("ownership-borrowed-close", analysis.Diagnostic{Pos: call.Pos(), Message: fmt.Sprintf("borrowed %s cannot be closed by the caller", receiverInfo.typeName)})
				return
			}
			if current.closed {
				diagnostic := analysis.Diagnostic{Pos: call.Pos(), Message: fmt.Sprintf("%s is closed more than once", receiverInfo.typeName)}
				if receiverInfo.created.IsValid() {
					diagnostic.Related = []analysis.RelatedInformation{{Pos: receiverInfo.created, Message: "handle originates here"}}
				}
				findings.add("ownership-double-close", diagnostic)
				return
			}
			if edge, retained := state.edges[receiverInfo.id]; retained {
				id := RuleID("ownership-retained-close")
				if edge.kind != "menu image" {
					id = "ownership-close-order"
				}
				findings.add(id, analysis.Diagnostic{Pos: call.Pos(), Message: fmt.Sprintf("%s cannot close while retained as %s", receiverInfo.typeName, edge.kind), Related: []analysis.RelatedInformation{{Pos: edge.pos, Message: "retaining operation is here"}}})
				return
			}
			current.closed = true
			state.handles[receiverInfo.id] = current
			deleteEdgesForOwner(state, receiverInfo.id)
			for id, info := range infos {
				if info.parent == receiverInfo.id {
					child := state.handles[id]
					child.closed = true
					state.handles[id] = child
				}
			}
			return
		}
		if current.closed {
			id := RuleID("ownership-use-after-close")
			if receiverInfo.typeName == "Bitmap" {
				id = "ownership-bitmap-use-after-close"
			}
			diagnostic := analysis.Diagnostic{Pos: call.Pos(), Message: fmt.Sprintf("%s.%s uses a handle after Close", receiverInfo.typeName, name)}
			if current.closedAt.IsValid() {
				diagnostic.Related = []analysis.RelatedInformation{{Pos: current.closedAt, Message: current.closedBy}}
			}
			findings.add(id, diagnostic)
		}
	}
	key := receiver + "." + name
	resultOwner := func() int {
		if call.Referrers() == nil {
			return 0
		}
		for _, ref := range *call.Referrers() {
			if extract, ok := ref.(*ssa.Extract); ok && extract.Index == 0 {
				return values[extract].id
			}
		}
		return 0
	}
	retain := func(index int, kind string) {
		if index < len(args) {
			if child, ok := values[args[index]]; ok {
				owner := 0
				if receiverKnown {
					owner = receiverInfo.id
				}
				state.edges[child.id] = retentionEdge{owner: owner, pos: call.Pos(), kind: kind}
			}
		}
	}
	clearKind := func(kind string) {
		for child, edge := range state.edges {
			if edge.kind == kind && (!receiverKnown || edge.owner == receiverInfo.id) {
				delete(state.edges, child)
			}
		}
	}
	switch key {
	case "SpriteTileMaps.NewSpriteTileMap":
		if len(args) > 0 {
			if child, ok := values[args[0]]; ok {
				state.edges[child.id] = retentionEdge{owner: resultOwner(), pos: call.Pos(), kind: "sprite tilemap table"}
			}
		}
	case "SamplePlayerFactory.NewSamplePlayer":
		if len(args) > 0 {
			if child, ok := values[args[0]]; ok {
				state.edges[child.id] = retentionEdge{owner: resultOwner(), pos: call.Pos(), kind: "player sample"}
			}
		}
	case "SystemControls.SetMenuImage":
		clearKind("menu image")
		retain(0, "menu image")
	case "SystemControls.ClearMenuImage":
		clearKind("menu image")
	case "BitmapDataGraphics.SetBitmapMask":
		clearKind("bitmap mask")
		retain(1, "bitmap mask")
	case "BitmapDataGraphics.ClearBitmapMask":
		clearKind("bitmap mask")
	case "Sprite.SetBitmap":
		clearKind("sprite image")
		retain(0, "sprite image")
	case "Sprite.SetStencilImage":
		clearKind("sprite stencil")
		retain(0, "sprite stencil")
	case "Sprite.ClearStencil":
		clearKind("sprite stencil")
	case "Sprite.SetTileMap":
		clearKind("sprite tilemap")
		retain(0, "sprite tilemap")
	case "Sprite.ClearTileMap":
		clearKind("sprite tilemap")
	case "VideoPlayer.SetContext":
		clearKind("video context")
		retain(0, "video context")
	case "VideoPlayer.UseScreenContext":
		clearKind("video context")
	case "SamplePlayerControls.SetSample":
		clearKind("player sample")
		retain(0, "player sample")
	case "AudioChannel.AddSource":
		retain(0, "audio channel source")
	case "AudioChannel.RemoveSource":
		clearArgumentEdge(args, values, state, 0)
	case "AudioChannel.AddEffect":
		retain(0, "audio channel effect")
	case "AudioChannel.RemoveEffect":
		clearArgumentEdge(args, values, state, 0)
	case "Instrument.AddVoice":
		retain(0, "instrument voice")
	case "SequenceTrack.SetInstrument":
		clearKind("track instrument")
		retain(0, "track instrument")
	case "Sequence.SetTrack":
		retain(1, "sequence track")
	}
	if strings.HasPrefix(name, "Set") && strings.HasSuffix(name, "Modulator") {
		clearKind(key)
		retain(len(args)-1, key)
	}
}

func clearArgumentEdge(args []ssa.Value, values map[ssa.Value]handleInfo, state *ownershipState, index int) {
	if index < len(args) {
		if info, ok := values[args[index]]; ok {
			delete(state.edges, info.id)
		}
	}
}

func deleteEdgesForOwner(state *ownershipState, owner int) {
	for child, edge := range state.edges {
		if edge.owner == owner {
			delete(state.edges, child)
		}
	}
}

func reportOwnershipLeaks(function *ssa.Function, infos map[int]handleInfo, state ownershipState, findings ownershipFindings) {
	for id, current := range state.handles {
		info := infos[id]
		if info.kind != handleOwned || !current.proven || current.closed || current.deferred || current.escaped {
			continue
		}
		findings.add("ownership-resource-leak", analysis.Diagnostic{Pos: info.created, Message: fmt.Sprintf("owned %s reaches a return from %s without Close or transfer", info.typeName, function.Name())})
	}
}

func cloneOwnershipState(source ownershipState) ownershipState {
	result := ownershipState{handles: make(map[int]handleState, len(source.handles)), edges: make(map[int]retentionEdge, len(source.edges))}
	for id, value := range source.handles {
		result.handles[id] = value
	}
	for id, value := range source.edges {
		result.edges[id] = value
	}
	return result
}

func ownershipStateKey(block *ssa.BasicBlock, state ownershipState) string {
	ids := make([]int, 0, len(state.handles))
	for id := range state.handles {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var key strings.Builder
	key.WriteString(strconv.Itoa(block.Index))
	key.WriteByte(':')
	for _, id := range ids {
		value := state.handles[id]
		key.WriteString(strconv.Itoa(id))
		if value.closed {
			key.WriteByte('c')
		}
		if value.deferred {
			key.WriteByte('d')
		}
		if value.escaped {
			key.WriteByte('e')
		}
		if value.proven {
			key.WriteByte('p')
		}
		if edge, ok := state.edges[id]; ok {
			key.WriteByte('r')
			key.WriteString(strconv.Itoa(edge.owner))
			key.WriteString(edge.kind)
		}
		key.WriteByte(';')
	}
	return key.String()
}
