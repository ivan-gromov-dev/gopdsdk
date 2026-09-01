package analyzer

import (
	"context"
	"errors"
	"go/ast"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
)

func TestStandardProvidersSupplyControlFlowSSAAndImportedFacts(t *testing.T) {
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{"./kernelprovider"}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	providers := StandardProviders()
	implementation := &analysis.Analyzer{
		Name: "providers", Doc: "exercise shared analyzer providers",
		Requires: []*analysis.Analyzer{providers.ControlFlow, providers.SSA},
		Run: func(pass *analysis.Pass) (any, error) {
			cfgs, ok := pass.ResultOf[providers.ControlFlow].(*ctrlflow.CFGs)
			if !ok {
				return nil, errors.New("control-flow result missing")
			}
			ssa, ok := pass.ResultOf[providers.SSA].(*buildssa.SSA)
			if !ok || len(ssa.SrcFuncs) == 0 {
				return nil, errors.New("SSA result missing")
			}
			for _, file := range pass.Files {
				for _, declaration := range file.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if ok && function.Name.Name == "Caller" {
						graph := cfgs.FuncDecl(function)
						if graph == nil || !graph.NoReturn() {
							return nil, errors.New("imported no-return fact was not applied")
						}
						pass.Reportf(function.Pos(), "providers ready")
						return nil, nil
					}
				}
			}
			return nil, errors.New("Caller function missing")
		},
	}
	registry := newTestRegistry(t, Registration{RuleID: "device-goroutine", Analyzer: implementation})
	result, err := registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-goroutine"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Message != "providers ready" {
		t.Fatalf("findings = %+v", result.Findings)
	}
}

func TestStandardProvidersHaveStableIdentity(t *testing.T) {
	first, second := StandardProviders(), StandardProviders()
	if first.Syntax != second.Syntax || first.ControlFlow != second.ControlFlow || first.SSA != second.SSA {
		t.Fatal("provider identities changed")
	}
	if first.SSA.Requires[0] != first.ControlFlow {
		t.Fatal("SSA does not use the shared control-flow provider")
	}
}
