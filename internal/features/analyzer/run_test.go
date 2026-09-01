package analyzer

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestRunCheckTextJSONAndFindingExit(t *testing.T) {
	root := checkFixture(t, "package game\nfunc bad() {}\n")
	catalog := syntheticProtocolCatalog(t)
	implementation := &analysis.Analyzer{Name: "protocolsynthetic", Doc: "exercise the check protocol", Run: func(pass *analysis.Pass) (any, error) {
		function := pass.Files[0].Decls[0].(*ast.FuncDecl)
		pass.Report(analysis.Diagnostic{Pos: function.Name.Pos(), End: function.Name.End(), Message: "synthetic finding", Category: "synthetic",
			Related:        []analysis.RelatedInformation{{Pos: function.Pos(), End: function.Name.Pos(), Message: "declaration starts here"}},
			SuggestedFixes: []analysis.SuggestedFix{{Message: "rename", TextEdits: []analysis.TextEdit{{Pos: function.Name.Pos(), End: function.Name.End(), NewText: []byte("good")}}}}})
		return nil, nil
	}}
	registry, err := NewRegistry(catalog, Registration{RuleID: "workspace-protocol-synthetic", Analyzer: implementation})
	if err != nil {
		t.Fatal(err)
	}
	options := CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", ModuleRoot: root}

	for _, test := range []struct{ format, contains string }{{"text", "warning workspace-protocol-synthetic: synthetic finding [device]"}, {"json", `"schema": "gopdsdk-check/v1"`}} {
		var stdout, stderr bytes.Buffer
		err := RunCheck(context.Background(), []string{"check", "--target", "device", "--format", test.format, "./..."}, &stdout, &stderr, options)
		var commandErr *CommandError
		if !errors.As(err, &commandErr) || commandErr.Code != ExitFindings {
			t.Fatalf("%s error = %v", test.format, err)
		}
		if !strings.Contains(stdout.String(), test.contains) {
			t.Fatalf("%s output missing %q:\n%s", test.format, test.contains, stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("%s stderr = %q", test.format, stderr.String())
		}
	}
}

func TestRunCheckCleanConfigurationLoadAndCancellationExits(t *testing.T) {
	root := checkFixture(t, "package game\n")
	catalog := syntheticProtocolCatalog(t)
	registry, err := NewRegistry(catalog)
	if err != nil {
		t.Fatal(err)
	}
	options := CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", ModuleRoot: root}
	var stdout, stderr bytes.Buffer
	if err := RunCheck(context.Background(), []string{"check", "--target", "shared"}, &stdout, &stderr, options); err != nil || stdout.String() != "No findings.\n" {
		t.Fatalf("clean check = %v, %q", err, stdout.String())
	}

	for _, arguments := range [][]string{{"check", "--format", "xml"}, {"check", "--target", "host"}, {"check", "--tags", "one,,two"}} {
		err := RunCheck(context.Background(), arguments, &bytes.Buffer{}, &bytes.Buffer{}, options)
		var commandErr *CommandError
		if !errors.As(err, &commandErr) || commandErr.Code != ExitConfiguration {
			t.Fatalf("%v error = %v", arguments, err)
		}
	}

	broken := checkFixture(t, "package game\nfunc broken(\n")
	options.ModuleRoot = broken
	err = RunCheck(context.Background(), []string{"check", "--target", "both"}, &bytes.Buffer{}, &bytes.Buffer{}, options)
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.Code != ExitPackageLoad {
		t.Fatalf("load error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = RunCheck(ctx, []string{"check"}, &bytes.Buffer{}, &bytes.Buffer{}, options)
	if !errors.As(err, &commandErr) || commandErr.Code != ExitCanceled {
		t.Fatalf("cancellation = %v", err)
	}
}

func TestRunCheckRepositoryConfigurationAndCLIOverride(t *testing.T) {
	root := checkFixture(t, "package game\nfunc bad() {}\n")
	writeCheckConfig(t, root, `{"schema":"gopdsdk-check-config/v1","format":"json","target":"device","patterns":["./..."]}`)
	catalog := syntheticProtocolCatalog(t)
	implementation := &analysis.Analyzer{Name: "protocolsynthetic", Doc: "exercise configured check", Run: func(pass *analysis.Pass) (any, error) {
		function := pass.Files[0].Decls[0].(*ast.FuncDecl)
		pass.Reportf(function.Name.Pos(), "configured finding")
		return nil, nil
	}}
	registry, err := NewRegistry(catalog, Registration{RuleID: "workspace-protocol-synthetic", Analyzer: implementation})
	if err != nil {
		t.Fatal(err)
	}
	options := CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", ModuleRoot: root}

	var configured bytes.Buffer
	err = RunCheck(context.Background(), []string{"check"}, &configured, &bytes.Buffer{}, options)
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.Code != ExitFindings || !strings.Contains(configured.String(), `"target": "device"`) {
		t.Fatalf("configured run = %v\n%s", err, configured.String())
	}
	var overridden bytes.Buffer
	if err := RunCheck(context.Background(), []string{"check", "--target", "shared", "--format", "text"}, &overridden, &bytes.Buffer{}, options); err != nil || overridden.String() != "No findings.\n" {
		t.Fatalf("CLI override = %v, %q", err, overridden.String())
	}
	var belowThreshold bytes.Buffer
	if err := RunCheck(context.Background(), []string{"check", "--target", "device", "--format", "text", "--severity", "workspace=information", "--fail-on", "warning"}, &belowThreshold, &bytes.Buffer{}, options); err != nil || !strings.Contains(belowThreshold.String(), "information workspace-protocol-synthetic") {
		t.Fatalf("below-threshold report = %v, %q", err, belowThreshold.String())
	}
	var excluded bytes.Buffer
	if err := RunCheck(context.Background(), []string{"check", "--target", "device", "--format", "text", "--exclude-rules", "workspace-protocol-synthetic"}, &excluded, &bytes.Buffer{}, options); err != nil || excluded.String() != "No findings.\n" {
		t.Fatalf("excluded rule = %v, %q", err, excluded.String())
	}
}

func checkFixture(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/game\n\ngo 1.26.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "game.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
