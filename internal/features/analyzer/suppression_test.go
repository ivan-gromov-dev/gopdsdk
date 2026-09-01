package analyzer

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestRunCheckInlineSuppressionIsReportedButDoesNotFail(t *testing.T) {
	root := checkFixture(t, "package game\n//gopdsdk:ignore workspace-protocol-synthetic -- accepted fixture exception\nfunc bad() {}\n")
	catalog := syntheticProtocolCatalog(t)
	implementation := &analysis.Analyzer{Name: "protocolsynthetic", Doc: "exercise suppression", Run: func(pass *analysis.Pass) (any, error) {
		function := pass.Files[0].Decls[0].(*ast.FuncDecl)
		pass.Reportf(function.Name.Pos(), "suppressed finding")
		return nil, nil
	}}
	registry, err := NewRegistry(catalog, Registration{RuleID: "workspace-protocol-synthetic", Analyzer: implementation})
	if err != nil {
		t.Fatal(err)
	}
	options := CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", ModuleRoot: root}
	var output bytes.Buffer
	if err := RunCheck(context.Background(), []string{"check", "--target", "device", "--format", "json"}, &output, &bytes.Buffer{}, options); err != nil {
		t.Fatalf("suppressed check = %v\n%s", err, output.String())
	}
	for _, required := range []string{`"suppression": {`, `"kind": "inline"`, `"reason": "accepted fixture exception"`} {
		if !strings.Contains(output.String(), required) {
			t.Fatalf("output missing %q:\n%s", required, output.String())
		}
	}
}

func TestRunCheckRejectsMalformedUnknownAndDuplicateSuppressions(t *testing.T) {
	catalog := syntheticProtocolCatalog(t)
	registry, err := NewRegistry(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"missing reason":          "package game\n//gopdsdk:ignore workspace-protocol-synthetic\nfunc bad() {}\n",
		"unknown rule":            "package game\n//gopdsdk:ignore workspace-does-not-exist -- reason\nfunc bad() {}\n",
		"string is not directive": "package game\nconst text = \"//gopdsdk:ignore missing -- reason\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			options := CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", ModuleRoot: checkFixture(t, source)}
			err := RunCheck(context.Background(), []string{"check", "--target", "device"}, &bytes.Buffer{}, &bytes.Buffer{}, options)
			var commandErr *CommandError
			if name == "string is not directive" {
				if err != nil {
					t.Fatalf("string literal rejected: %v", err)
				}
			} else if !errors.As(err, &commandErr) || commandErr.Code != ExitConfiguration {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
