package analyzer

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// Registration attaches one rule identity to its go/analysis implementation.
type Registration struct {
	RuleID   RuleID
	Analyzer *analysis.Analyzer
}

// Registry is an immutable set of analyzer implementations. It is safe to run
// against independent package snapshots concurrently.
type Registry struct {
	catalog       RuleCatalog
	registrations map[RuleID]*analysis.Analyzer
}

// Finding is a normalized analyzer diagnostic. Step 2 adds the stable CLI and
// structured protocol around this kernel-owned record.
type Finding struct {
	RuleID      RuleID
	Analyzer    string
	PackageID   string
	Target      Target
	Position    string
	End         string
	Category    string
	Message     string
	URL         string
	Related     []RelatedFinding
	Fixes       []FindingFix
	Suppression *FindingSuppression
}

type FindingSuppression struct{ Kind, Reason string }

// RelatedFinding adds a secondary source range to a finding.
type RelatedFinding struct {
	Position string
	End      string
	Message  string
}

// FindingFix is one analyzer-provided edit group. Consumers must apply every
// edit in a group together.
type FindingFix struct {
	Message string
	Edits   []FindingEdit
}

// FindingEdit replaces one normalized source range with NewText.
type FindingEdit struct {
	Position string
	End      string
	NewText  string
}

// RunResult contains findings in deterministic rule, package, and source order.
type RunResult struct {
	Findings []Finding
}

// ExecutionFailure identifies an analyzer implementation failure separately
// from package load errors preserved by Snapshot.
type ExecutionFailure struct {
	RuleID    RuleID
	Analyzer  string
	PackageID string
	Err       error
}

// ExecutionError deterministically groups failures from a parallel run.
type ExecutionError struct {
	Failures []ExecutionFailure
}

func (err *ExecutionError) Error() string {
	parts := make([]string, 0, len(err.Failures))
	for _, failure := range err.Failures {
		parts = append(parts, fmt.Sprintf("rule %q analyzer %q on package %q: %v", failure.RuleID, failure.Analyzer, failure.PackageID, failure.Err))
	}
	return "run analyzers: " + strings.Join(parts, "; ")
}

func (err *ExecutionError) Unwrap() []error {
	result := make([]error, 0, len(err.Failures))
	for _, failure := range err.Failures {
		result = append(result, failure.Err)
	}
	return result
}

// NewRegistry validates registrations and their prerequisite graph.
func NewRegistry(catalog RuleCatalog, registrations ...Registration) (Registry, error) {
	registered := make(map[RuleID]*analysis.Analyzer, len(registrations))
	identities := make(map[*analysis.Analyzer]RuleID, len(registrations))
	roots := make([]*analysis.Analyzer, 0, len(registrations))
	for _, registration := range registrations {
		if _, exists := catalog.Rule(registration.RuleID); !exists {
			return Registry{}, fmt.Errorf("register unknown analyzer rule %q", registration.RuleID)
		}
		if registration.Analyzer == nil {
			return Registry{}, fmt.Errorf("register analyzer rule %q with nil implementation", registration.RuleID)
		}
		if _, exists := registered[registration.RuleID]; exists {
			return Registry{}, fmt.Errorf("register analyzer rule %q more than once", registration.RuleID)
		}
		if prior, exists := identities[registration.Analyzer]; exists {
			return Registry{}, fmt.Errorf("register analyzer %q for both rules %q and %q", registration.Analyzer.Name, prior, registration.RuleID)
		}
		registered[registration.RuleID] = registration.Analyzer
		identities[registration.Analyzer] = registration.RuleID
		roots = append(roots, registration.Analyzer)
	}
	if err := analysis.Validate(roots); err != nil {
		return Registry{}, fmt.Errorf("validate analyzer registry: %w", err)
	}
	return Registry{catalog: catalog, registrations: registered}, nil
}

// Run executes the selected registered analyzers. Packages run concurrently;
// analyzer prerequisites within one package run once in dependency order.
func (registry Registry) Run(ctx context.Context, snapshot Snapshot, selection RuleSelection) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	rules, err := registry.catalog.Select(selection)
	if err != nil {
		return RunResult{}, err
	}
	selected := make(map[RuleID]bool, len(rules))
	roots := make(map[*analysis.Analyzer]RuleID, len(rules))
	for _, rule := range rules {
		if !containsTarget(rule.Targets, snapshot.Target) {
			continue
		}
		if analyzer := registry.registrations[rule.ID]; analyzer != nil {
			selected[rule.ID] = true
			roots[analyzer] = rule.ID
		}
	}
	if len(roots) == 0 {
		return RunResult{}, nil
	}

	executor := newExecutor(ctx, snapshot)
	var group sync.WaitGroup
	for _, loaded := range snapshot.Loaded {
		loaded := loaded
		for analyzer := range roots {
			analyzer := analyzer
			group.Add(1)
			go func() { defer group.Done(); executor.execute(analyzer, loaded) }()
		}
	}
	group.Wait()
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	var result RunResult
	var failures []ExecutionFailure
	for _, loaded := range snapshot.Loaded {
		for analyzer, ruleID := range roots {
			out := executor.result(analyzer, loaded)
			if out.err != nil {
				if !errors.Is(out.err, context.Canceled) && !errors.Is(out.err, context.DeadlineExceeded) {
					failures = append(failures, ExecutionFailure{RuleID: ruleID, Analyzer: analyzer.Name, PackageID: loaded.ID, Err: out.err})
				}
				continue
			}
			if selected[ruleID] {
				for _, diagnostic := range out.diagnostics {
					result.Findings = append(result.Findings, normalizeDiagnostic(snapshot, loaded, ruleID, analyzer.Name, diagnostic))
				}
			}
		}
	}
	sortFindings(result.Findings)
	sort.Slice(failures, func(i, j int) bool {
		left, right := failures[i], failures[j]
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		if left.PackageID != right.PackageID {
			return left.PackageID < right.PackageID
		}
		return left.Analyzer < right.Analyzer
	})
	if len(failures) != 0 {
		return RunResult{}, &ExecutionError{Failures: failures}
	}
	return result, nil
}

type passResult struct {
	value       any
	diagnostics []analysis.Diagnostic
	err         error
}

type executionKey struct {
	analyzer *analysis.Analyzer
	pkg      *packages.Package
}
type executionState struct {
	ready  chan struct{}
	result passResult
}

type executor struct {
	ctx        context.Context
	snapshot   Snapshot
	mu         sync.Mutex
	executions map[executionKey]*executionState
	facts      factStore
}

func newExecutor(ctx context.Context, snapshot Snapshot) *executor {
	return &executor{ctx: ctx, snapshot: snapshot, executions: make(map[executionKey]*executionState), facts: newFactStore()}
}

func (executor *executor) result(analyzer *analysis.Analyzer, pkg *packages.Package) passResult {
	executor.mu.Lock()
	state := executor.executions[executionKey{analyzer, pkg}]
	executor.mu.Unlock()
	if state == nil {
		return passResult{err: errors.New("analyzer execution was not scheduled")}
	}
	<-state.ready
	return state.result
}

func (executor *executor) execute(analyzer *analysis.Analyzer, loaded *packages.Package) passResult {
	key := executionKey{analyzer, loaded}
	executor.mu.Lock()
	if state := executor.executions[key]; state != nil {
		executor.mu.Unlock()
		<-state.ready
		return state.result
	}
	state := &executionState{ready: make(chan struct{})}
	executor.executions[key] = state
	executor.mu.Unlock()
	out := executor.run(analyzer, loaded)
	executor.mu.Lock()
	state.result = out
	close(state.ready)
	executor.mu.Unlock()
	return out
}

func (executor *executor) run(analyzer *analysis.Analyzer, loaded *packages.Package) (out passResult) {
	if err := executor.ctx.Err(); err != nil {
		out.err = err
		return out
	}
	inputs := make(map[*analysis.Analyzer]any, len(analyzer.Requires))
	for _, required := range analyzer.Requires {
		dependency := executor.execute(required, loaded)
		if dependency.err != nil {
			out.err = fmt.Errorf("required analyzer %q failed: %w", required.Name, dependency.err)
			return out
		}
		inputs[required] = dependency.value
	}
	if len(analyzer.FactTypes) != 0 {
		imports := make([]*packages.Package, 0, len(loaded.Imports))
		for _, imported := range loaded.Imports {
			imports = append(imports, imported)
		}
		sort.Slice(imports, func(i, j int) bool { return imports[i].ID < imports[j].ID })
		for _, imported := range imports {
			if dependency := executor.execute(analyzer, imported); dependency.err != nil {
				out.err = fmt.Errorf("analyze imported package %q for facts: %w", imported.ID, dependency.err)
				return out
			}
		}
	}
	if loaded.IllTyped && !analyzer.RunDespiteErrors {
		return out
	}
	pass := &analysis.Pass{Analyzer: analyzer, Fset: loaded.Fset, Files: loaded.Syntax, OtherFiles: loaded.OtherFiles, IgnoredFiles: loaded.IgnoredFiles,
		Pkg: loaded.Types, TypesInfo: loaded.TypesInfo, TypesSizes: loaded.TypesSizes, ResultOf: inputs, TypeErrors: loaded.TypeErrors,
		Module: analysisModule(loaded.Module), ReadFile: executor.snapshot.readFile}
	pass.Report = func(diagnostic analysis.Diagnostic) { out.diagnostics = append(out.diagnostics, diagnostic) }
	attachPassContext(pass, executor.ctx)
	executor.facts.attach(pass)
	defer func() {
		executor.facts.finish(pass)
		detachPassContext(pass)
		if recovered := recover(); recovered != nil {
			out.err = fmt.Errorf("panic: %v", recovered)
		}
		if out.err == nil && analyzer.ResultType != nil && reflect.TypeOf(out.value) != analyzer.ResultType {
			out.err = fmt.Errorf("returned result type %T, want %v", out.value, analyzer.ResultType)
		}
	}()
	out.value, out.err = analyzer.Run(pass)
	return out
}

type objectFactKey struct {
	analyzer *analysis.Analyzer
	object   types.Object
	kind     reflect.Type
}
type packageFactKey struct {
	analyzer *analysis.Analyzer
	pkg      *types.Package
	kind     reflect.Type
}
type factStore struct {
	mu       sync.RWMutex
	objects  map[objectFactKey]analysis.Fact
	packages map[packageFactKey]analysis.Fact
	active   map[*analysis.Pass]bool
}

func newFactStore() factStore {
	return factStore{objects: make(map[objectFactKey]analysis.Fact), packages: make(map[packageFactKey]analysis.Fact), active: make(map[*analysis.Pass]bool)}
}

func (store *factStore) attach(pass *analysis.Pass) {
	store.mu.Lock()
	store.active[pass] = true
	store.mu.Unlock()
	pass.ImportObjectFact = func(object types.Object, fact analysis.Fact) bool { return store.importObject(pass, object, fact) }
	pass.ImportPackageFact = func(pkg *types.Package, fact analysis.Fact) bool { return store.importPackage(pass, pkg, fact) }
	pass.ExportObjectFact = func(object types.Object, fact analysis.Fact) { store.exportObject(pass, object, fact) }
	pass.ExportPackageFact = func(fact analysis.Fact) { store.exportPackage(pass, fact) }
	pass.AllObjectFacts = func() []analysis.ObjectFact { return store.allObjectFacts(pass) }
	pass.AllPackageFacts = func() []analysis.PackageFact { return store.allPackageFacts(pass) }
}

func (store *factStore) finish(pass *analysis.Pass) {
	store.mu.Lock()
	delete(store.active, pass)
	store.mu.Unlock()
}

func (store *factStore) check(pass *analysis.Pass, fact analysis.Fact) reflect.Type {
	if fact == nil || reflect.TypeOf(fact).Kind() != reflect.Pointer {
		panic("analysis fact must be a non-nil pointer")
	}
	store.mu.RLock()
	active := store.active[pass]
	store.mu.RUnlock()
	if !active {
		panic("analysis fact accessed after pass completion")
	}
	kind := reflect.TypeOf(fact)
	for _, declared := range pass.Analyzer.FactTypes {
		if reflect.TypeOf(declared) == kind {
			return kind
		}
	}
	panic(fmt.Sprintf("analyzer %q used undeclared fact type %v", pass.Analyzer.Name, kind))
}

func cloneFact(fact analysis.Fact) analysis.Fact {
	copy := reflect.New(reflect.TypeOf(fact).Elem())
	copy.Elem().Set(reflect.ValueOf(fact).Elem())
	return copy.Interface().(analysis.Fact)
}

func (store *factStore) importObject(pass *analysis.Pass, object types.Object, fact analysis.Fact) bool {
	kind := store.check(pass, fact)
	store.mu.RLock()
	stored := store.objects[objectFactKey{pass.Analyzer, object, kind}]
	store.mu.RUnlock()
	if stored == nil {
		return false
	}
	reflect.ValueOf(fact).Elem().Set(reflect.ValueOf(stored).Elem())
	return true
}
func (store *factStore) importPackage(pass *analysis.Pass, pkg *types.Package, fact analysis.Fact) bool {
	kind := store.check(pass, fact)
	store.mu.RLock()
	stored := store.packages[packageFactKey{pass.Analyzer, pkg, kind}]
	store.mu.RUnlock()
	if stored == nil {
		return false
	}
	reflect.ValueOf(fact).Elem().Set(reflect.ValueOf(stored).Elem())
	return true
}
func (store *factStore) exportObject(pass *analysis.Pass, object types.Object, fact analysis.Fact) {
	kind := store.check(pass, fact)
	if object == nil || object.Pkg() != pass.Pkg {
		panic("analysis object fact belongs to another package")
	}
	store.mu.Lock()
	store.objects[objectFactKey{pass.Analyzer, object, kind}] = cloneFact(fact)
	store.mu.Unlock()
}
func (store *factStore) exportPackage(pass *analysis.Pass, fact analysis.Fact) {
	kind := store.check(pass, fact)
	store.mu.Lock()
	store.packages[packageFactKey{pass.Analyzer, pass.Pkg, kind}] = cloneFact(fact)
	store.mu.Unlock()
}
func (store *factStore) allObjectFacts(pass *analysis.Pass) []analysis.ObjectFact {
	store.mu.RLock()
	defer store.mu.RUnlock()
	var result []analysis.ObjectFact
	for key, fact := range store.objects {
		if key.analyzer == pass.Analyzer && visiblePackage(pass.Pkg, key.object.Pkg()) {
			result = append(result, analysis.ObjectFact{Object: key.object, Fact: cloneFact(fact)})
		}
	}
	return result
}
func (store *factStore) allPackageFacts(pass *analysis.Pass) []analysis.PackageFact {
	store.mu.RLock()
	defer store.mu.RUnlock()
	var result []analysis.PackageFact
	for key, fact := range store.packages {
		if key.analyzer == pass.Analyzer && visiblePackage(pass.Pkg, key.pkg) {
			result = append(result, analysis.PackageFact{Package: key.pkg, Fact: cloneFact(fact)})
		}
	}
	return result
}
func visiblePackage(current, candidate *types.Package) bool {
	if current == candidate {
		return true
	}
	for _, imported := range current.Imports() {
		if imported == candidate {
			return true
		}
	}
	return false
}

func (snapshot Snapshot) readFile(name string) ([]byte, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	absolute = filepath.Clean(absolute)
	if content, ok := snapshot.overlay[absolute]; ok {
		return append([]byte(nil), content...), nil
	}
	return os.ReadFile(absolute)
}

func normalizeDiagnostic(snapshot Snapshot, loaded *packages.Package, ruleID RuleID, analyzerName string, diagnostic analysis.Diagnostic) Finding {
	position := loaded.Fset.PositionFor(diagnostic.Pos, true)
	end := loaded.Fset.PositionFor(diagnostic.End, true)
	finding := Finding{RuleID: ruleID, Analyzer: analyzerName, PackageID: loaded.ID, Target: snapshot.Target,
		Position: normalizeTokenPosition(snapshot.ModuleRoot, position), End: normalizeTokenPosition(snapshot.ModuleRoot, end),
		Category: diagnostic.Category, Message: diagnostic.Message, URL: diagnostic.URL}
	for _, related := range diagnostic.Related {
		finding.Related = append(finding.Related, RelatedFinding{
			Position: normalizeTokenPosition(snapshot.ModuleRoot, loaded.Fset.PositionFor(related.Pos, true)),
			End:      normalizeTokenPosition(snapshot.ModuleRoot, loaded.Fset.PositionFor(related.End, true)),
			Message:  related.Message,
		})
	}
	for _, fix := range diagnostic.SuggestedFixes {
		normalized := FindingFix{Message: fix.Message}
		for _, edit := range fix.TextEdits {
			normalized.Edits = append(normalized.Edits, FindingEdit{
				Position: normalizeTokenPosition(snapshot.ModuleRoot, loaded.Fset.PositionFor(edit.Pos, true)),
				End:      normalizeTokenPosition(snapshot.ModuleRoot, loaded.Fset.PositionFor(edit.End, true)),
				NewText:  string(edit.NewText),
			})
		}
		sort.Slice(normalized.Edits, func(i, j int) bool {
			if normalized.Edits[i].Position != normalized.Edits[j].Position {
				return normalized.Edits[i].Position < normalized.Edits[j].Position
			}
			if normalized.Edits[i].End != normalized.Edits[j].End {
				return normalized.Edits[i].End < normalized.Edits[j].End
			}
			return normalized.Edits[i].NewText < normalized.Edits[j].NewText
		})
		finding.Fixes = append(finding.Fixes, normalized)
	}
	sort.Slice(finding.Related, func(i, j int) bool {
		if finding.Related[i].Position != finding.Related[j].Position {
			return finding.Related[i].Position < finding.Related[j].Position
		}
		if finding.Related[i].End != finding.Related[j].End {
			return finding.Related[i].End < finding.Related[j].End
		}
		return finding.Related[i].Message < finding.Related[j].Message
	})
	sort.Slice(finding.Fixes, func(i, j int) bool { return finding.Fixes[i].Message < finding.Fixes[j].Message })
	return finding
}

func normalizeTokenPosition(root string, position token.Position) string {
	if !position.IsValid() {
		return ""
	}
	return normalizePosition(root, position.String())
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		if left.PackageID != right.PackageID {
			return left.PackageID < right.PackageID
		}
		if left.Position != right.Position {
			return left.Position < right.Position
		}
		if left.End != right.End {
			return left.End < right.End
		}
		return left.Message < right.Message
	})
}

func containsTarget(targets []Target, target Target) bool {
	for _, candidate := range targets {
		if candidate == target {
			return true
		}
	}
	return false
}

func analysisModule(module *packages.Module) *analysis.Module {
	if module == nil {
		return nil
	}
	result := &analysis.Module{Path: module.Path, Version: module.Version, Time: module.Time, Main: module.Main, Indirect: module.Indirect, Dir: module.Dir, GoMod: module.GoMod, GoVersion: module.GoVersion}
	if module.Replace != nil {
		result.Replace = analysisModule(module.Replace)
	}
	if module.Error != nil {
		result.Error = &analysis.ModuleError{Err: module.Error.Err}
	}
	return result
}
