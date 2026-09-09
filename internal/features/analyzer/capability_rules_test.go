package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestExternalCapabilityRules(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "gopdsdk")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.CommandContext(context.Background(), "go", "build", "-buildvcs=false", "-o", binary, "./cmd/gopdsdk")
	build.Dir = repositoryRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/capability\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	for _, target := range []string{"shared", "simulator", "device"} {
		for _, suppressed := range []bool{false, true} {
			comment := ""
			if suppressed {
				comment = "//gopdsdk:ignore capability-unchecked-assertion -- fixture caller contract\n"
			}
			writeAnalyzerFixture(t, root, "game.go", "package game\nimport \""+playdatePackage+"\"\nfunc f(c playdate.Context){\n"+comment+"c.(playdate.Launcher).ExitToLauncher()\n}\n")
			command := exec.CommandContext(context.Background(), binary, "check", "--target", target, "--format", "json", "--categories", "capability", ".")
			command.Dir = root
			output, err := command.Output()
			if suppressed {
				if err != nil {
					t.Fatalf("suppressed: %v\n%s", err, output)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != ExitFindings {
					t.Fatalf("finding exit: %v\n%s", err, output)
				}
			}
			var report Report
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatal(err)
			}
			if len(report.Diagnostics) != 1 || report.Diagnostics[0].Rule != "capability-unchecked-assertion" || (report.Diagnostics[0].Suppression != nil) != suppressed {
				t.Fatalf("report: %+v", report)
			}
		}
	}
	writeAnalyzerFixture(t, root, "game.go", "package game\nimport \""+playdatePackage+"\"\nfunc f(c playdate.Context){if v,ok:=c.(playdate.Videos);ok{_,_=v.LoadVideo(\"movie.pdv\")}}\n")
	command := exec.CommandContext(context.Background(), binary, "check", "--target", "device", "--format", "json", "--categories", "capability", "--gopdsdk-floor", "v0.9.0", "--playdate-sdk", "3.0.0", ".")
	command.Dir = root
	output, err := command.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != ExitFindings {
		t.Fatalf("availability exit: %v\n%s", err, output)
	}
	var report Report
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Rule != "capability-video-availability" {
		t.Fatalf("availability report: %+v", report)
	}
}

func TestCapabilityRules(t *testing.T) {
	for _, test := range []struct {
		name, body string
		want       []RuleID
	}{
		{"unchecked", `func f(c playdate.Context){c.(playdate.Launcher).ExitToLauncher()}`, []RuleID{"capability-unchecked-assertion"}},
		{"guarded private callee", `func use(c playdate.Context){c.(playdate.Launcher).ExitToLauncher()};func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);ok{use(c)}}`, nil},
		{"mixed private callers", `func use(c playdate.Context){c.(playdate.Launcher).ExitToLauncher()};func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);ok{use(c)}};func unsafe(c playdate.Context){use(c)}`, []RuleID{"capability-unproven-assertion"}},
		{"escaped private callee", `func use(c playdate.Context){c.(playdate.Launcher).ExitToLauncher()};var escape=use;func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);ok{use(c)}}`, []RuleID{"capability-unproven-assertion"}},
		{"required on init", lifecycleCapabilityFixture + `func(*game)Init(c playdate.Context)error{if _,ok:=c.(playdate.Launcher);!ok{return errors.New("missing")};return nil}`, nil},
		{"init fallback", lifecycleCapabilityFixture + `func(*game)Init(c playdate.Context)error{if _,ok:=c.(playdate.Launcher);!ok{return nil};return nil}`, []RuleID{"capability-unproven-assertion"}},
		{"init private sentinel", lifecycleCapabilityFixture + `var missing=errors.New("missing");func(*game)Init(c playdate.Context)error{if _,ok:=c.(playdate.Launcher);!ok{return missing};return nil}`, nil},
		{"init mutable sentinel", lifecycleCapabilityFixture + `var missing=errors.New("missing");func clear(){missing=nil};func(*game)Init(c playdate.Context)error{if _,ok:=c.(playdate.Launcher);!ok{return missing};return nil}`, []RuleID{"capability-unproven-assertion"}},
		{"manual callback contexts", lifecycleCapabilityFixture + `func(*game)Init(c playdate.Context)error{if _,ok:=c.(playdate.Launcher);!ok{return errors.New("missing")};return nil};func manual(c playdate.Context){g:=&game{};g.Update(c)}`, []RuleID{"capability-unproven-assertion"}},
		{"cross function uncertainty", `func checked(c playdate.Context){_,_=c.(playdate.Launcher)};func use(c playdate.Context){c.(playdate.Launcher).ExitToLauncher()}`, []RuleID{"capability-unproven-assertion"}},
		{"helper interface conversion", `func has(c playdate.Context)bool{_,ok:=any(c).(playdate.Launcher);return ok};func f(c playdate.Context){if has(c){c.(playdate.Launcher).ExitToLauncher()}}`, nil},
		{"closure enclosing guard", `func f(c playdate.Context)func(){if _,ok:=c.(playdate.Launcher);ok{return func(){c.(playdate.Launcher).ExitToLauncher()}};return nil}`, nil},
		{"closure mutable capture", `func f(c,other playdate.Context)func(){if _,ok:=c.(playdate.Launcher);ok{callback:=func(){c.(playdate.Launcher).ExitToLauncher()};c=other;return callback};return nil}`, []RuleID{"capability-unchecked-assertion"}},
		{"explicit bool guard", `func f(c playdate.Context){_,ok:=c.(playdate.Launcher);if ok==true{c.(playdate.Launcher).ExitToLauncher()}}`, nil},
		{"nil interface", `func f(){var c any;c.(playdate.Launcher).ExitToLauncher()}`, []RuleID{"capability-impossible-assertion"}},
		{"comma ok", `func f(c playdate.Context){if p,ok:=c.(playdate.Launcher);ok{p.ExitToLauncher()}}`, nil},
		{"guard original", `func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);ok{c.(playdate.Launcher).ExitToLauncher()}}`, nil},
		{"early return", `func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);!ok{return};c.(playdate.Launcher).ExitToLauncher()}`, nil},
		{"false branch", `func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);!ok{c.(playdate.Launcher).ExitToLauncher()}}`, []RuleID{"capability-impossible-assertion"}},
		{"redundant", `func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);ok{_,_=c.(playdate.Launcher)}}`, []RuleID{"capability-redundant-check"}},
		{"contradictory", `func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);!ok{_,_=c.(playdate.Launcher)}}`, []RuleID{"capability-redundant-check"}},
		{"switch", `func f(c playdate.Context){switch c.(type){case playdate.Launcher:c.(playdate.Launcher).ExitToLauncher();default:return}}`, nil},
		{"helper", `func has(c playdate.Context)bool{_,ok:=c.(playdate.Launcher);return ok};func f(c playdate.Context){if has(c){c.(playdate.Launcher).ExitToLauncher()}}`, nil},
		{"negated helper", `func missing(c playdate.Context)bool{_,ok:=c.(playdate.Launcher);return !ok};func f(c playdate.Context){if missing(c){return};c.(playdate.Launcher).ExitToLauncher()}`, nil},
		{"required helper", `func require(c playdate.Context)(playdate.Launcher,error){p,ok:=c.(playdate.Launcher);if !ok{return nil,errMissing};return p,nil};var errMissing error;func f(c playdate.Context){p,err:=require(c);if err==nil{p.ExitToLauncher()}}`, nil},
		{"reassigned", `func f(c,other playdate.Context){if _,ok:=c.(playdate.Launcher);ok{c=other;c.(playdate.Launcher).ExitToLauncher()}}`, []RuleID{"capability-unchecked-assertion"}},
		{"unknown factory", `var replace func()playdate.Context;func f(c playdate.Context){if _,ok:=c.(playdate.Launcher);ok{c=replace();c.(playdate.Launcher).ExitToLauncher()}}`, []RuleID{"capability-unchecked-assertion"}},
		{"unknown predicate", `var has func(playdate.Context)bool;func f(c playdate.Context){if has(c){c.(playdate.Launcher).ExitToLauncher()}}`, []RuleID{"capability-unchecked-assertion"}},
		{"closure checked", `func f(c playdate.Context)func(){return func(){if p,ok:=c.(playdate.Launcher);ok{p.ExitToLauncher()}}}`, nil},
		{"loop checked", `func f(values []playdate.Context){for _,c:=range values{if _,ok:=c.(playdate.Launcher);!ok{continue};c.(playdate.Launcher).ExitToLauncher()}}`, nil},
		{"wrapper", `type both interface{playdate.Launcher;playdate.PowerMonitor};func f(c playdate.Context){if _,ok:=c.(both);ok{c.(playdate.Launcher).ExitToLauncher()}}`, nil},
		{"mandatory", `func f(c playdate.Context){c.(playdate.System).CurrentTimeMilliseconds()}`, nil},
		{"boxed impossible", `func f(){any(1).(playdate.Launcher).ExitToLauncher()}`, []RuleID{"capability-impossible-assertion"}},
		{"boxed supported", `type launcher struct{};func(launcher)ExitToLauncher(){};func f(){any(launcher{}).(playdate.Launcher).ExitToLauncher()}`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/capability\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
			writeAnalyzerFixture(t, root, "game.go", "package game\nimport \""+playdatePackage+"\"\n"+test.body)
			snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: TargetSimulator})
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
			result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyCapability}})
			if err != nil {
				t.Fatal(err)
			}
			var got []RuleID
			for _, finding := range result.Findings {
				got = append(got, finding.RuleID)
				if len(finding.Fixes) != 0 {
					t.Fatal("capability checks must not generate panicking quick fixes")
				}
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("findings: %+v; want %v", result.Findings, test.want)
			}
		})
	}
}

func TestCapabilityRuleImportsExportedHelperFact(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/capability\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "guards/guards.go", "package guards\nimport \""+playdatePackage+"\"\nfunc HasLauncher(c playdate.Context)bool{_,ok:=c.(playdate.Launcher);return ok}\n")
	writeAnalyzerFixture(t, root, "game.go", "package game\nimport (\""+playdatePackage+"\";\"example.com/capability/guards\")\nfunc f(c playdate.Context){if guards.HasLauncher(c){c.(playdate.Launcher).ExitToLauncher()}}\n")
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: TargetSimulator, Patterns: []string{"./..."}})
	if err != nil {
		t.Fatal(err)
	}
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyCapability}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("cross-package guard findings: %+v", result.Findings)
	}
}

func TestCapabilityAvailabilityVersionsAndTargets(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/availability\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", "package game\nimport \""+playdatePackage+"\"\nfunc f(c playdate.Context){if v,ok:=c.(playdate.Videos);ok{_,_=v.LoadVideo(\"movie.pdv\")}}\n")
	for _, test := range []struct {
		name, floor, sdk string
		target           Target
		want             int
	}{
		{"current", "v1.0.0", "3.1.1", TargetSimulator, 0},
		{"older gopdsdk", "v0.9.0", "3.1.1", TargetSimulator, 1},
		{"older official SDK", "v1.0.0", "3.0.0", TargetDevice, 1},
		{"shared target", "v0.9.0", "3.0.0", TargetShared, 0},
		{"unconfigured", "", "", TargetDevice, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: test.target, Patterns: []string{"."}})
			if err != nil {
				t.Fatal(err)
			}
			got := capabilityAvailabilityFindings(snapshot, test.floor, test.sdk)
			if len(got) != test.want {
				t.Fatalf("findings: %+v; want %d", got, test.want)
			}
		})
	}
}

const lifecycleCapabilityFixture = `import "errors"
var _=errors.New
type game struct{}
func New()playdate.Game{return &game{}}
func(*game)Update(c playdate.Context)(bool,error){c.(playdate.Launcher).ExitToLauncher();return true,nil}
`
