package analyzer

import (
	"context"
	"errors"
	"fmt"
	"go/token"
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
	RuleID    RuleID
	Analyzer  string
	PackageID string
	Target    Target
	Position  string
	End       string
	Category  string
	Message   string
	URL       string
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

// NewRegistry validates registrations and their prerequisite graph. Fact-based
// analyzers are intentionally rejected until the Step 1 fact provider is added.
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
	visited := make(map[*analysis.Analyzer]bool)
	var rejectFacts func(*analysis.Analyzer) error
	rejectFacts = func(analyzer *analysis.Analyzer) error {
		if visited[analyzer] {
			return nil
		}
		visited[analyzer] = true
		if len(analyzer.FactTypes) != 0 {
			return fmt.Errorf("analyzer %q requires the Step 1 fact provider", analyzer.Name)
		}
		for _, required := range analyzer.Requires {
			if err := rejectFacts(required); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := rejectFacts(root); err != nil {
			return Registry{}, err
		}
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

	type packageResult struct {
		findings []Finding
		failures []ExecutionFailure
	}
	results := make(chan packageResult, len(snapshot.Loaded))
	var group sync.WaitGroup
	for _, loaded := range snapshot.Loaded {
		loaded := loaded
		group.Add(1)
		go func() {
			defer group.Done()
			results <- runPackage(ctx, snapshot, loaded, roots, selected)
		}()
	}
	group.Wait()
	close(results)
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	var result RunResult
	var failures []ExecutionFailure
	for packageResult := range results {
		result.Findings = append(result.Findings, packageResult.findings...)
		failures = append(failures, packageResult.failures...)
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

func runPackage(ctx context.Context, snapshot Snapshot, loaded *packages.Package, roots map[*analysis.Analyzer]RuleID, selected map[RuleID]bool) (result struct {
	findings []Finding
	failures []ExecutionFailure
}) {
	completed := make(map[*analysis.Analyzer]passResult)
	var execute func(*analysis.Analyzer) passResult
	execute = func(analyzer *analysis.Analyzer) (out passResult) {
		if prior, ok := completed[analyzer]; ok {
			return prior
		}
		if err := ctx.Err(); err != nil {
			out.err = err
			completed[analyzer] = out
			return out
		}
		inputs := make(map[*analysis.Analyzer]any, len(analyzer.Requires))
		for _, required := range analyzer.Requires {
			dependency := execute(required)
			if dependency.err != nil {
				out.err = fmt.Errorf("required analyzer %q failed: %w", required.Name, dependency.err)
				completed[analyzer] = out
				return out
			}
			inputs[required] = dependency.value
		}
		if loaded.IllTyped && !analyzer.RunDespiteErrors {
			completed[analyzer] = out
			return out
		}
		pass := &analysis.Pass{
			Analyzer: analyzer, Fset: loaded.Fset, Files: loaded.Syntax,
			OtherFiles: loaded.OtherFiles, IgnoredFiles: loaded.IgnoredFiles,
			Pkg: loaded.Types, TypesInfo: loaded.TypesInfo, TypesSizes: loaded.TypesSizes,
			ResultOf: inputs, TypeErrors: loaded.TypeErrors, Module: analysisModule(loaded.Module),
			ReadFile: snapshot.readFile,
		}
		pass.Report = func(diagnostic analysis.Diagnostic) { out.diagnostics = append(out.diagnostics, diagnostic) }
		defer func() {
			if recovered := recover(); recovered != nil {
				out.err = fmt.Errorf("panic: %v", recovered)
			}
			if out.err == nil && analyzer.ResultType != nil && reflect.TypeOf(out.value) != analyzer.ResultType {
				out.err = fmt.Errorf("returned result type %T, want %v", out.value, analyzer.ResultType)
			}
			completed[analyzer] = out
		}()
		out.value, out.err = analyzer.Run(pass)
		return out
	}

	for analyzer, ruleID := range roots {
		out := execute(analyzer)
		if out.err != nil {
			if !errors.Is(out.err, context.Canceled) && !errors.Is(out.err, context.DeadlineExceeded) {
				result.failures = append(result.failures, ExecutionFailure{RuleID: ruleID, Analyzer: analyzer.Name, PackageID: loaded.ID, Err: out.err})
			}
			continue
		}
		if selected[ruleID] {
			for _, diagnostic := range out.diagnostics {
				result.findings = append(result.findings, normalizeDiagnostic(snapshot, loaded, ruleID, analyzer.Name, diagnostic))
			}
		}
	}
	return result
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
	return Finding{RuleID: ruleID, Analyzer: analyzerName, PackageID: loaded.ID, Target: snapshot.Target,
		Position: normalizeTokenPosition(snapshot.ModuleRoot, position), End: normalizeTokenPosition(snapshot.ModuleRoot, end),
		Category: diagnostic.Category, Message: diagnostic.Message, URL: diagnostic.URL}
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
