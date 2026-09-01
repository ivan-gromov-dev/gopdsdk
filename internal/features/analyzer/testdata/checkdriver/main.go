// Command checkdriver is a test-only external process for the analyzer command boundary.
package main

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"os"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/analyzer"
	"golang.org/x/tools/go/analysis"
)

func main() {
	catalog, err := analyzer.NewRuleCatalog(analyzer.ContractInventory())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(analyzer.ExitInternal)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mode := os.Getenv("GOPDSDK_CHECKDRIVER_MODE")
	implementation := &analysis.Analyzer{Name: "checkdriver", Doc: "test external command outcomes", Run: func(pass *analysis.Pass) (any, error) {
		switch mode {
		case "finding":
			for _, declaration := range pass.Files[0].Decls {
				if function, ok := declaration.(*ast.FuncDecl); ok {
					pass.Reportf(function.Name.Pos(), "external synthetic finding")
					break
				}
			}
			return nil, nil
		case "failure":
			return nil, errors.New("external synthetic analyzer failure")
		case "cancel":
			cancel()
			<-analyzer.PassContext(pass).Done()
			return nil, analyzer.PassContext(pass).Err()
		default:
			return nil, nil
		}
	}}
	registry, err := analyzer.NewRegistry(catalog, analyzer.Registration{RuleID: "device-goroutine", Analyzer: implementation})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(analyzer.ExitInternal)
	}
	err = analyzer.RunCheck(ctx, os.Args[1:], os.Stdout, os.Stderr, analyzer.CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", ModuleRoot: "."})
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "checkdriver:", err)
	if coded, ok := err.(interface{ ExitCode() int }); ok {
		os.Exit(coded.ExitCode())
	}
	os.Exit(analyzer.ExitInternal)
}
