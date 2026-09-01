package analyzer

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestRegistryRunsDependenciesOnceAndSortsFindings(t *testing.T) {
	snapshot := loadRegistryFixture(t)
	dependencyRuns := 0
	dependency := &analysis.Analyzer{
		Name: "names", Doc: "collect package name", ResultType: reflect.TypeOf(""),
		Run: func(pass *analysis.Pass) (any, error) {
			dependencyRuns++
			return pass.Pkg.Name(), nil
		},
	}
	ruleAnalyzer := &analysis.Analyzer{
		Name: "devicegoroutine", Doc: "test device goroutine rule", Requires: []*analysis.Analyzer{dependency},
		Run: func(pass *analysis.Pass) (any, error) {
			if pass.ResultOf[dependency] != "compliant" {
				return nil, errors.New("missing dependency result")
			}
			for index := len(pass.Files) - 1; index >= 0; index-- {
				pass.Reportf(pass.Files[index].Pos(), "file %d", index)
			}
			return nil, nil
		},
	}
	registry := newTestRegistry(t, Registration{RuleID: "device-goroutine", Analyzer: ruleAnalyzer})
	first, err := registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-goroutine"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-goroutine"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("runs differ:\n%+v\n%+v", first, second)
	}
	if dependencyRuns != 2 {
		t.Fatalf("dependency runs = %d, want once per run", dependencyRuns)
	}
	if len(first.Findings) == 0 {
		t.Fatal("run produced no findings")
	}
	for index, finding := range first.Findings {
		if finding.RuleID != "device-goroutine" || finding.Target != TargetDevice || strings.Contains(finding.Position, "\\") || strings.Contains(strings.ToLower(finding.Position), strings.ToLower(snapshot.ModuleRoot)) {
			t.Fatalf("finding %d is not normalized: %+v", index, finding)
		}
		if index > 0 && first.Findings[index-1].Position > finding.Position {
			t.Fatalf("findings are not sorted: %+v", first.Findings)
		}
	}
}

func TestRegistryUsesOverlayForReadFile(t *testing.T) {
	const overlayText = "package compliant\n// unsaved editor content\ntype Game struct{}\n"
	snapshot, err := LoadPackages(context.Background(), LoadConfig{
		ModuleRoot: contractFixtureRoot(), Patterns: []string{"./compliant"}, Target: TargetDevice,
		Overlay: map[string][]byte{"compliant/game.go": []byte(overlayText)},
	})
	if err != nil {
		t.Fatal(err)
	}
	implementation := &analysis.Analyzer{Name: "overlay", Doc: "test overlays", Run: func(pass *analysis.Pass) (any, error) {
		name := pass.Fset.PositionFor(pass.Files[0].Pos(), true).Filename
		content, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(string(content), "unsaved editor content") {
			return nil, errors.New("overlay content missing")
		}
		return nil, nil
	}}
	registry := newTestRegistry(t, Registration{RuleID: "device-goroutine", Analyzer: implementation})
	if _, err := registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-goroutine"}}); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrySeparatesExecutionFailureAndCancellation(t *testing.T) {
	snapshot := loadRegistryFixture(t)
	broken := &analysis.Analyzer{Name: "broken", Doc: "fail predictably", Run: func(*analysis.Pass) (any, error) { return nil, errors.New("broken rule") }}
	registry := newTestRegistry(t, Registration{RuleID: "device-goroutine", Analyzer: broken})
	_, err := registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-goroutine"}})
	var executionError *ExecutionError
	if !errors.As(err, &executionError) || len(executionError.Failures) != 1 || !strings.Contains(err.Error(), "broken rule") {
		t.Fatalf("Run() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := registry.Run(ctx, snapshot, RuleSelection{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Run() = %v", err)
	}
}

func TestRegistrySkipsWrongTargetAndIllTypedPackage(t *testing.T) {
	shared, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{"./compliant"}, Target: TargetShared})
	if err != nil {
		t.Fatal(err)
	}
	runs := 0
	implementation := &analysis.Analyzer{Name: "deviceonly", Doc: "device only", Run: func(*analysis.Pass) (any, error) { runs++; return nil, nil }}
	registry := newTestRegistry(t, Registration{RuleID: "device-goroutine", Analyzer: implementation})
	if _, err := registry.Run(context.Background(), shared, RuleSelection{IDs: []RuleID{"device-goroutine"}}); err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("device analyzer ran %d times for shared target", runs)
	}

	partial, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{"./compliant"}, Target: TargetDevice, Overlay: map[string][]byte{"compliant/game.go": []byte("package compliant\nfunc broken(")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Run(context.Background(), partial, RuleSelection{IDs: []RuleID{"device-goroutine"}}); err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("analyzer ran %d times for ill-typed package", runs)
	}
}

func TestNewRegistryValidatesBoundary(t *testing.T) {
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	valid := &analysis.Analyzer{Name: "valid", Doc: "valid test analyzer", Run: func(*analysis.Pass) (any, error) { return nil, nil }}
	for name, registrations := range map[string][]Registration{
		"unknown rule":       {{RuleID: "device-unknown", Analyzer: valid}},
		"nil implementation": {{RuleID: "device-goroutine"}},
		"duplicate rule":     {{RuleID: "device-goroutine", Analyzer: valid}, {RuleID: "device-goroutine", Analyzer: valid}},
		"duplicate analyzer": {{RuleID: "device-goroutine", Analyzer: valid}, {RuleID: "device-channel", Analyzer: valid}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRegistry(catalog, registrations...); err == nil {
				t.Fatal("NewRegistry() succeeded")
			}
		})
	}
}

func loadRegistryFixture(t *testing.T) Snapshot {
	t.Helper()
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{"./compliant"}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func newTestRegistry(t *testing.T, registrations ...Registration) Registry {
	t.Helper()
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(catalog, registrations...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}
