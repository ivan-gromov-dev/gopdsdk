package analyzer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestRunCheckBaselineSuppressesAndDetectsStaleEntries(t *testing.T) {
	root := checkFixture(t, "package game\nfunc bad() {}\n")
	catalog := syntheticProtocolCatalog(t)
	implementation := &analysis.Analyzer{Name: "protocolsynthetic", Doc: "exercise baseline", Run: func(pass *analysis.Pass) (any, error) {
		function := pass.Files[0].Decls[0].(*ast.FuncDecl)
		pass.Reportf(function.Name.Pos(), "baseline finding")
		return nil, nil
	}}
	registry, err := NewRegistry(catalog, Registration{RuleID: "workspace-protocol-synthetic", Analyzer: implementation})
	if err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(root, "baseline.json")
	writeBaseline(t, baselinePath, "baseline finding", 2)
	options := CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", ModuleRoot: root}
	var output bytes.Buffer
	if err := RunCheck(context.Background(), []string{"check", "--target", "device", "--format", "json", "--baseline", "baseline.json"}, &output, &bytes.Buffer{}, options); err != nil {
		t.Fatalf("baseline check = %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), `"kind": "baseline"`) || !strings.Contains(output.String(), `"reason": "accepted migration debt"`) {
		t.Fatalf("baseline suppression missing:\n%s", output.String())
	}

	writeBaseline(t, baselinePath, "baseline finding", 3)
	err = RunCheck(context.Background(), []string{"check", "--target", "device", "--baseline", "baseline.json"}, &bytes.Buffer{}, &bytes.Buffer{}, options)
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.Code != ExitConfiguration || !strings.Contains(err.Error(), "stale baseline entries") {
		t.Fatalf("stale baseline = %v", err)
	}
}

func TestBaselineRejectsUnknownDuplicateAndUnsafeEntries(t *testing.T) {
	catalog := syntheticProtocolCatalog(t)
	valid := BaselineEntry{Rule: "workspace-protocol-synthetic", Target: TargetDevice, Path: "game.go", Line: 2, Column: 6, Message: "finding", Reason: "migration"}
	for name, baseline := range map[string]Baseline{
		"schema":         {Schema: "gopdsdk-check-baseline/v2"},
		"unknown rule":   {Schema: BaselineSchema, Entries: []BaselineEntry{{Rule: "workspace-missing", Target: TargetDevice, Path: "game.go", Line: 1, Column: 1, Message: "finding", Reason: "migration"}}},
		"unsafe path":    {Schema: BaselineSchema, Entries: []BaselineEntry{{Rule: valid.Rule, Target: valid.Target, Path: "../game.go", Line: 1, Column: 1, Message: "finding", Reason: "migration"}}},
		"backslash path": {Schema: BaselineSchema, Entries: []BaselineEntry{{Rule: valid.Rule, Target: valid.Target, Path: `dir\game.go`, Line: 1, Column: 1, Message: "finding", Reason: "migration"}}},
		"duplicate":      {Schema: BaselineSchema, Entries: []BaselineEntry{valid, valid}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := baseline.Validate(catalog); err == nil {
				t.Fatal("invalid baseline succeeded")
			}
		})
	}
}

func writeBaseline(t *testing.T, name, message string, line int) {
	t.Helper()
	content := []byte(`{"schema":"gopdsdk-check-baseline/v1","entries":[{"rule":"workspace-protocol-synthetic","target":"device","path":"game.go","line":` +
		fmt.Sprint(line) + `,"column":6,"message":"` + message + `","reason":"accepted migration debt"}]}`)
	if err := os.WriteFile(name, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
