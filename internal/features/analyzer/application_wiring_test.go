package analyzer

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/runtime/simabi"
)

// This is generated factory wiring plus Go runtime dispatch acceptance. Native
// context construction, cgo, SDK compilation, and Simulator execution are not
// exercised: the platform context and before-init hook are replaced with nil.
func TestApplicationGeneratedFactoryDispatch(t *testing.T) {
	const module = "github.com/ivan-gromov-dev/gopdsdk/analyzerfixture"
	sources, err := simabi.Render(simabi.Config{APIHeader: "pd_api.h", RuntimeImport: "github.com/ivan-gromov-dev/gopdsdk/internal/features/runtime", PlaydateImport: playdatePackage, ApplicationImport: module})
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "simulator.go", sources.Go, 0)
	if err != nil {
		t.Fatal(err)
	}
	var factory ast.Expr
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "NewApplication" {
			return true
		}
		if len(call.Args) != 3 {
			t.Fatal("generated NewApplication arguments changed")
		}
		factory = call.Args[0]
		return false
	})
	if factory == nil {
		t.Fatal("generated source has no NewApplication factory")
	}
	var expression bytes.Buffer
	if err := printer.Fprint(&expression, fset, factory); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, result     string
		events, findings int
	}{
		{"pointer", "&game{}", 1, 0}, {"value", "game{}", 0, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "wiring"), 0755); err != nil {
				t.Fatal(err)
			}
			writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module %s\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", module, filepath.ToSlash(repositoryRoot(t))))
			writeAnalyzerFixture(t, root, "pdxinfo", "name=Generated entry fixture\n")
			writeAnalyzerFixture(t, root, "game.go", applicationFixture+fmt.Sprintf(`
var Events int
func New() playdate.Game { return %s }
func (*game) HandleLifecycle(_ playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate { Events++ }; return nil }
`, test.result))
			writeAnalyzerFixture(t, root, "wiring/wiring_test.go", fmt.Sprintf(`package wiring
import("testing"; app %q; sdkRuntime "github.com/ivan-gromov-dev/gopdsdk/internal/features/runtime")
func TestGeneratedFactory(t *testing.T){
 application,err:=sdkRuntime.NewApplication(%s,nil,nil); if err!=nil{t.Fatal(err)}
 if err:=application.Handle(sdkRuntime.EventInit,0);err!=nil{t.Fatal(err)}
 if _,err:=application.Update(sdkRuntime.RawInput{});err!=nil{t.Fatal(err)}
 if err:=application.Handle(sdkRuntime.EventTerminate,0);err!=nil{t.Fatal(err)}
 if app.Events!=%d{t.Fatalf("lifecycle deliveries=%%d",app.Events)}
}`, module, expression.String(), test.events))
			command := exec.CommandContext(context.Background(), "go", "test", "./wiring")
			command.Dir = root
			command.Env = environmentWith("GOWORK", "off")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("generated factory dispatch: %v\n%s", err, output)
			}
			snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Patterns: []string{"."}, Target: TargetSimulator})
			if err != nil {
				t.Fatal(err)
			}
			if snapshotHasLoadErrors(snapshot) {
				t.Fatalf("load: %+v", snapshot.Packages)
			}
			options, err := DefaultCheckOptions()
			if err != nil {
				t.Fatal(err)
			}
			result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"application-lifecycle-shape"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Findings) != test.findings {
				t.Fatalf("findings: %+v", result.Findings)
			}
		})
	}
}
