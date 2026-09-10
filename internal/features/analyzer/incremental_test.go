package analyzer

import (
	"context"
	"errors"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestIncrementalEngineOverlayCacheAndBatchEquivalence(t *testing.T) {
	root := incrementalWorkspace(t)
	engine, catalog, registry := incrementalTestEngine(t, 2)
	request := IncrementalRequest{LoadConfig: LoadConfig{ModuleRoot: root, Target: TargetDevice, Overlay: map[string][]byte{"game.go": []byte("package game\nfunc bad() {}\n")}}, Version: 1, Schedule: ScheduleFastEdit,
		Selection: RuleSelection{IDs: []RuleID{"device-goroutine"}}}
	first, err := engine.Analyze(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Findings) != 1 || first.Metrics.CacheMisses != 1 {
		t.Fatalf("first result = %#v", first)
	}
	request.Version = 2
	second, err := engine.Analyze(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Metrics.CacheHits != 1 || !reflect.DeepEqual(first.Findings, second.Findings) {
		t.Fatalf("cached result = %#v", second)
	}

	snapshot, err := LoadPackages(context.Background(), request.LoadConfig)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := registry.Run(context.Background(), snapshot, request.Selection)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second.Findings, batch.Findings) {
		t.Fatalf("incremental findings differ from batch\nincremental: %#v\nbatch: %#v", second.Findings, batch.Findings)
	}
	_ = catalog
}

func TestIncrementalEngineEditSequenceAndStaleRejection(t *testing.T) {
	root := incrementalWorkspace(t)
	engine, _, _ := incrementalTestEngine(t, 1)
	base := IncrementalRequest{LoadConfig: LoadConfig{ModuleRoot: root, Target: TargetDevice}, Schedule: ScheduleSave,
		Selection: RuleSelection{IDs: []RuleID{"device-goroutine"}}}
	base.Version = 2
	base.Overlay = map[string][]byte{"game.go": []byte("package game\nfunc bad() {}\n")}
	result, err := engine.Analyze(context.Background(), base)
	if err != nil || len(result.Findings) != 1 {
		t.Fatalf("edited result = %#v, %v", result, err)
	}
	base.Version = 1
	if _, err := engine.Analyze(context.Background(), base); !errors.Is(err, ErrStaleAnalysis) {
		t.Fatalf("stale error = %v", err)
	}
	base.Version = 3
	base.Overlay = map[string][]byte{"game.go": []byte("package game\nfunc renamed() {}\n")}
	result, err = engine.Analyze(context.Background(), base)
	if err != nil || len(result.Findings) != 0 || result.Metrics.CacheMisses != 1 || result.Metrics.InvalidatedItems == 0 {
		t.Fatalf("renamed result = %#v, %v", result, err)
	}
	base.Version = 4
	base.Overlay = map[string][]byte{"game.go": []byte("package game\nfunc (")}
	result, err = engine.Analyze(context.Background(), base)
	if err != nil || len(result.LoadErrors) == 0 {
		t.Fatalf("syntax-error result = %#v, %v", result, err)
	}
}

func TestIncrementalFingerprintTracksConfigurationAndFiles(t *testing.T) {
	root := incrementalWorkspace(t)
	request := IncrementalRequest{LoadConfig: LoadConfig{ModuleRoot: root, Target: TargetDevice}, Schedule: ScheduleFastEdit}
	first, err := incrementalKey(context.Background(), root, request)
	if err != nil {
		t.Fatal(err)
	}
	request.BuildTags = []string{"extra"}
	configured, err := incrementalKey(context.Background(), root, request)
	if err != nil || configured == first {
		t.Fatalf("configuration key = %q, %v", configured, err)
	}
	created := filepath.Join(root, "created.go")
	if err := os.WriteFile(created, []byte("package game\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withFile, err := incrementalKey(context.Background(), root, request)
	if err != nil || withFile == configured {
		t.Fatalf("created-file key = %q, %v", withFile, err)
	}
	if err := os.Remove(created); err != nil {
		t.Fatal(err)
	}
	deleted, err := incrementalKey(context.Background(), root, request)
	if err != nil || deleted != configured {
		t.Fatalf("deleted-file key = %q want %q, %v", deleted, configured, err)
	}
}

func TestIncrementalEngineCancellationAndConcurrentRequests(t *testing.T) {
	root := incrementalWorkspace(t)
	engine, _, _ := incrementalTestEngine(t, 2)
	request := IncrementalRequest{LoadConfig: LoadConfig{ModuleRoot: root, Target: TargetDevice}, Version: 1, Schedule: ScheduleFastEdit,
		Selection: RuleSelection{IDs: []RuleID{"device-goroutine"}}}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	canceledResult, err := engine.Analyze(canceled, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if canceledResult.Version != request.Version {
		t.Fatalf("cancellation result = %#v", canceledResult)
	}
	var group sync.WaitGroup
	for version := uint64(2); version < 10; version++ {
		version := version
		group.Add(1)
		go func() {
			defer group.Done()
			copy := request
			copy.Version = version
			_, err := engine.Analyze(context.Background(), copy)
			if err != nil && !errors.Is(err, ErrStaleAnalysis) {
				t.Errorf("version %d: %v", version, err)
			}
		}()
	}
	group.Wait()
}

func incrementalWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/game\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "game.go"), []byte("package game\nfunc good() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func incrementalTestEngine(t *testing.T, limit int) (*IncrementalEngine, RuleCatalog, Registry) {
	t.Helper()
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	implementation := &analysis.Analyzer{Name: "incrementaltest", Doc: "test incremental snapshots", Run: func(pass *analysis.Pass) (any, error) {
		for _, file := range pass.Files {
			ast.Inspect(file, func(node ast.Node) bool {
				if identifier, ok := node.(*ast.Ident); ok && identifier.Name == "bad" {
					pass.Reportf(identifier.Pos(), "bad identifier")
				}
				return true
			})
		}
		return nil, nil
	}}
	registry, err := NewRegistry(catalog, Registration{RuleID: "device-goroutine", Analyzer: implementation})
	if err != nil {
		t.Fatal(err)
	}
	return NewIncrementalEngine(catalog, registry, IncrementalOptions{MaxSnapshots: limit}), catalog, registry
}
