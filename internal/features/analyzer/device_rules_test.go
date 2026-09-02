package analyzer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestDeviceRulePackReportsDocumentedProfileViolations(t *testing.T) {
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{"./noncompliant"}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) {
		t.Fatalf("fixture load errors: %+v", snapshot.Packages)
	}
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(catalog, deviceRuleRegistrations()...)
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyDevice}})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[RuleID]bool)
	for _, finding := range result.Findings {
		seen[finding.RuleID] = true
	}
	var got []RuleID
	for rule := range seen {
		got = append(got, rule)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	want := []RuleID{"device-channel", "device-compiler-directive", "device-encoding-json", "device-finalizer", "device-fmt", "device-goroutine", "device-panic-cleanup", "device-recover", "device-reflect-symbol", "device-runtime-control", "device-select", "device-time-runtime"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reported rules = %v, want %v", got, want)
	}
}

func TestDeviceRulePackAcceptsDocumentedSubsetAndOtherTargets(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		pattern string
		target  Target
	}{
		{name: "documented device subset", pattern: "./compliant", target: TargetDevice},
		{name: "noncompliant code is valid for simulator", pattern: "./noncompliant", target: TargetSimulator},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{test.pattern}, Target: test.target})
			if err != nil {
				t.Fatal(err)
			}
			result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyDevice}})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Findings) != 0 {
				t.Fatalf("findings = %+v, want none", result.Findings)
			}
		})
	}
}

func TestDeviceCgoRulePrecedesToolchainLoadFailure(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{
		ModuleRoot: contractFixtureRoot(), Patterns: []string{"./noncompliant"}, BuildTags: []string{"analyzer_cgo"}, Target: TargetDevice,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) {
		t.Fatalf("device cgo preflight left load errors: %+v", snapshot.Packages)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-cgo"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || result.Findings[0].RuleID != "device-cgo" || !strings.HasSuffix(result.Findings[0].Position, "noncompliant/cgo.go:6:8") {
		t.Fatalf("cgo findings = %+v", result.Findings)
	}
}

func TestDefaultCheckOptionsRegistersEveryInitialDeviceRule(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := options.Catalog.Select(RuleSelection{Families: []RuleFamily{FamilyDevice}})
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.Family == FamilyDevice && options.Registry.registrations[rule.ID] == nil {
			t.Errorf("device rule %q is not registered", rule.ID)
		}
	}
}

func TestDeviceRulesReportForbiddenSymbolReferencesAndChannelRange(t *testing.T) {
	root := checkFixture(t, `package game
import (
	"reflect"
	"runtime"
	"time"
)
var _ = time.Now
var _ *time.Timer
var _ = reflect.MakeFunc
var _ = runtime.LockOSThread
func drain(channel <-chan int) { for range channel {} }
`)
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-channel", "device-reflect-symbol", "device-runtime-control", "device-time-runtime"}})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[RuleID]int)
	for _, finding := range result.Findings {
		seen[finding.RuleID]++
	}
	if seen["device-time-runtime"] != 2 || seen["device-reflect-symbol"] != 1 || seen["device-runtime-control"] != 1 || seen["device-channel"] != 2 {
		t.Fatalf("finding counts = %v", seen)
	}
}

func TestDeviceRuleCommandSourcePolicies(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	root := checkFixture(t, "package game\n")
	options.ModuleRoot = root
	writeAnalyzerFixture(t, root, "generated.go", "// Code generated by analyzer fixture. DO NOT EDIT.\n\npackage game\nfunc generated() { go func() {}() }\n")
	writeAnalyzerFixture(t, root, "tagged.go", "//go:build analyzer_extra\n\npackage game\nfunc tagged() { go func() {}() }\n")
	writeAnalyzerFixture(t, root, "game_test.go", "package game\nfunc testOnly() { go func() {}() }\n")

	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-goroutine"}, ExitSuccess, "No findings.")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-goroutine", "--generated", "include"}, ExitFindings, "generated.go")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-goroutine", "--tags", "analyzer_extra"}, ExitFindings, "tagged.go")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-goroutine", "--tests"}, ExitFindings, "game_test.go")
}

func TestDeviceRuleCommandSuppressionAndBothTargets(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = checkFixture(t, "package game\nfunc bad() {\n//gopdsdk:ignore device-goroutine -- fixture exception\ngo func() {}()\n}\n")
	assertDeviceCheck(t, options, []string{"check", "--target", "both", "--rules", "device-goroutine"}, ExitSuccess, "suppressed inline: fixture exception")
}

func TestDeviceRulesReportShortestKnownDependencyCallPaths(t *testing.T) {
	root := checkFixture(t, `package game
import "example.com/game/dependency"
func run() {
	_ = dependency.Safe(1)
	_ = dependency.Format(2)
	_ = dependency.Decode(nil)
}
`)
	if err := os.MkdirAll(filepath.Join(root, "dependency"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeAnalyzerFixture(t, root, "dependency/dependency.go", `package dependency
import (
	"encoding/json"
	"fmt"
	"strconv"
)
func Safe(value int) string { return strconv.Itoa(value) }
func format(value int) string { return fmt.Sprint(value) }
func Format(value int) string { return format(value) }
func Decode(data []byte) error { return json.Unmarshal(data, new(any)) }
`)
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = root
	var stdout bytes.Buffer
	err = RunCheck(context.Background(), []string{"check", "--target", "device", "--rules", "device-fmt,device-encoding-json", "."}, &stdout, &bytes.Buffer{}, options)
	var commandError *CommandError
	if !errors.As(err, &commandError) || commandError.Code != ExitFindings {
		t.Fatalf("RunCheck() = %v", err)
	}
	output := stdout.String()
	for _, wanted := range []string{"dependency.Format", "dependency.format", "fmt.Sprint", "dependency.Decode", "encoding/json.Unmarshal"} {
		if !strings.Contains(output, wanted) {
			t.Errorf("output missing %q:\n%s", wanted, output)
		}
	}
	if strings.Contains(output, "dependency.Safe") || strings.Count(output, "call reaches unavailable device feature") != 2 {
		t.Fatalf("unexpected reachable-path diagnostics:\n%s", output)
	}
}

func TestDeviceReachabilityInitializersAndGenericCalls(t *testing.T) {
	root := checkFixture(t, `package game
import "example.com/game/dependency"
var plain = dependency.Format(1)
var grouped = (dependency.Format)(2)
var generic = dependency.Generic[int](3)
var multiple = dependency.Pair[int, string](4, "value")
var method = dependency.Box[int]{}.Format(5)
var closure = func() string { return dependency.Generic(6) }
var safe = dependency.Safe(7)
var reference = dependency.Format
func run() { _ = dependency.Through(8) }
type formatter interface { Format(int) string }
func dynamic(value formatter) { _ = value.Format(9) }
`)
	if err := os.MkdirAll(filepath.Join(root, "dependency"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeAnalyzerFixture(t, root, "dependency/dependency.go", `package dependency
import "fmt"
func Format(value int) string { return fmt.Sprint(value) }
func Generic[T any](value T) string { return fmt.Sprint(value) }
func Pair[A, B any](a A, b B) string { return fmt.Sprint(a, b) }
type Box[T any] struct {}
func (Box[T]) Format(value T) string { return fmt.Sprint(value) }
func Through(value int) string { return Generic[int](value) }
func Safe(value int) int { return value }
`)
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Patterns: []string{"."}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) {
		t.Fatalf("fixture load errors: %+v", snapshot.Packages)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-fmt"}})
	if err != nil {
		t.Fatal(err)
	}
	wantLines := map[int]string{3: "Format", 4: "Format", 5: "Generic", 6: "Pair", 7: "Format", 8: "Generic", 11: "Through"}
	if len(result.Findings) != len(wantLines) {
		t.Fatalf("findings = %+v, want %d", result.Findings, len(wantLines))
	}
	for line, name := range wantLines {
		matched := false
		for _, finding := range result.Findings {
			if strings.Contains(finding.Position, fmt.Sprintf("game.go:%d:", line)) && strings.Contains(finding.Message, name) && strings.Contains(finding.Message, "fmt.Sprint") {
				matched = true
			}
		}
		if !matched {
			t.Errorf("missing %s call on line %d: %+v", name, line, result.Findings)
		}
	}
}

func TestDeviceReachabilityDependencyInitialization(t *testing.T) {
	root := checkFixture(t, `package game
import (
	_ "example.com/game/bridge"
	alias "example.com/game/variable"
	. "example.com/game/immediate"
	_ "example.com/game/safe"
)
var _ = alias.Value
var _ = Value
`)
	fixtures := map[string]string{
		"bridge": `package bridge
import _ "example.com/game/startup"
`,
		"startup": `package startup
import ("fmt"; "encoding/json")
func init() { longer() }
func init() { shortB(); shortA() }
func longer() { shortA() }
func shortA() { fmt.Sprint(1) }
func shortB() { fmt.Sprint(2) }
func init() { _ = json.Unmarshal(nil, new(any)) }
`,
		"variable": `package variable
import "fmt"
var Value = format[int](1)
func format[T any](value T) string { return fmt.Sprint(value) }
`,
		"immediate": `package immediate
import "fmt"
var Value = (func() string { return fmt.Sprint(1) })()
`,
		"safe": `package safe
import "fmt"
var stored = func() string { return fmt.Sprint(1) }
var returned = factory()
func factory() func() string { return func() string { return fmt.Sprint(2) } }
func Unused() string { return fmt.Sprint(3) }
func init() { _ = stored; _ = returned }
`,
	}
	for name, source := range fixtures {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		writeAnalyzerFixture(t, root, name+"/dependency.go", source)
	}
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = root
	var stdout bytes.Buffer
	err = RunCheck(context.Background(), []string{"check", "--target", "device", "--rules", "device-fmt,device-encoding-json", "."}, &stdout, &bytes.Buffer{}, options)
	var commandError *CommandError
	if !errors.As(err, &commandError) || commandError.Code != ExitFindings {
		t.Fatalf("RunCheck() = %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"game.go:3:4", "game.go:4:8", "game.go:5:4",
		"example.com/game/bridge initialization -> example.com/game/startup initialization",
		"startup.shortA", "fmt.Sprint", "encoding/json.Unmarshal", "variable.format", "immediate initialization",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"startup.longer", "startup.shortB", "game/safe", "game.go:6:"} {
		if strings.Contains(output, unwanted) {
			t.Errorf("unexpected %q:\n%s", unwanted, output)
		}
	}
	if strings.Count(output, "import initializes unavailable device feature") != 4 || strings.Contains(output, "call reaches unavailable device feature") {
		t.Fatalf("unexpected initialization findings:\n%s", output)
	}
	assertDeviceCheck(t, options, []string{"check", "--target", "simulator", "--rules", "device-fmt,device-encoding-json", "."}, ExitSuccess, "No findings.")
}

func TestDeviceReachabilityImmutableLocalFunctionValues(t *testing.T) {
	root := checkFixture(t, `package game
import "example.com/game/dependency"
func direct() { f := dependency.Format; _ = f(1) }
func aliases() { var f = dependency.Format; g := f; _ = (g)(2) }
func generic() { f := dependency.Generic[int]; _ = f(3) }
func method() { f := dependency.Box[int]{}.Format; _ = f(4) }
func wrapped() { _ = dependency.Through(5) }
func reassigned() { f := dependency.Format; f = dependency.Safe; _ = f(6) }
func escaped() { f := dependency.Format; p := &f; *p = dependency.Safe; _ = f(7) }
func captured() { f := dependency.Format; change := func() { f = dependency.Safe }; change(); _ = f(8) }
func shadowed() { f := dependency.Format; { f := dependency.Safe; _ = f(9) }; _ = f }
func ranged() { f := dependency.Format; for _, f = range []func(int) string{dependency.Safe} { _ = f(10) } }
func parameter(f func(int) string) { _ = f(11) }
var global = dependency.Format
func globalCall() { _ = global(12) }
type formatter interface { Format(int) string }
func iface(value formatter) { f := value.Format; _ = f(13) }
func unused() { f := dependency.Format; _ = f }
`)
	if err := os.MkdirAll(filepath.Join(root, "dependency"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeAnalyzerFixture(t, root, "dependency/dependency.go", `package dependency
import "fmt"
func Format(value int) string { f := fmt.Sprint; return f(value) }
func Generic[T any](value T) string { return fmt.Sprint(value) }
type Box[T any] struct {}
func (Box[T]) Format(value T) string { return fmt.Sprint(value) }
func Through(value int) string { f := Format; g := f; return g(value) }
func Safe(value int) string { return "safe" }
`)
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Patterns: []string{"."}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) {
		t.Fatalf("fixture load errors: %+v", snapshot.Packages)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{"device-fmt"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 5 {
		t.Fatalf("findings = %+v, want five immutable calls", result.Findings)
	}
	for index, finding := range result.Findings {
		if !strings.Contains(finding.Position, fmt.Sprintf("game.go:%d:", index+3)) || !strings.Contains(finding.Message, "fmt.Sprint") {
			t.Errorf("unexpected finding: %+v", finding)
		}
	}
}

func TestDeviceReachabilityInvokedLocalClosures(t *testing.T) {
	root := checkFixture(t, `package game
import (
	"example.com/game/dependency"
	_ "example.com/game/startup"
)
func run() {
	dependency.Run()
	dependency.Nested()
	dependency.Deferred()
	_ = dependency.Factory()
	dependency.Unused()
	dependency.Reassigned()
	dependency.Escaped()
}
`)
	fixtures := map[string]string{
		"dependency": `package dependency
import "fmt"
func Run() { format := fmt.Sprint; f := func() { format(1) }; alias := f; alias() }
func Nested() {
	f := func() { fmt.Sprint(2) }
	g := func() { f(); f() }
	h := func() { g(); g() }
	h()
}
func Deferred() { f := func() { fmt.Sprint(3) }; defer f() }
func Factory() func() { f := func() { fmt.Sprint(4) }; return f }
func Unused() { f := func() { fmt.Sprint(5) }; _ = f }
func Reassigned() { f := func() { fmt.Sprint(6) }; f = func() {}; f() }
func Escaped() { f := func() { fmt.Sprint(7) }; p := &f; *p = func() {}; f() }
`,
		"startup": `package startup
import "encoding/json"
var initialized = initialize()
func initialize() bool {
	f := func() { _ = json.Unmarshal(nil, new(any)) }
	g := f
	g()
	return true
}
`,
	}
	for name, source := range fixtures {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		writeAnalyzerFixture(t, root, name+"/dependency.go", source)
	}
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = root
	var stdout bytes.Buffer
	err = RunCheck(context.Background(), []string{"check", "--target", "device", "--rules", "device-fmt,device-encoding-json", "."}, &stdout, &bytes.Buffer{}, options)
	var commandError *CommandError
	if !errors.As(err, &commandError) || commandError.Code != ExitFindings {
		t.Fatalf("RunCheck() = %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"dependency.Run", "dependency.Nested", "dependency.Deferred", "fmt.Sprint", "startup initialization", "startup.initialize", "encoding/json.Unmarshal", "game.go:4:4"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"dependency.Factory", "dependency.Unused", "dependency.Reassigned", "dependency.Escaped"} {
		if strings.Contains(output, unwanted) {
			t.Errorf("unexpected %q:\n%s", unwanted, output)
		}
	}
	if strings.Count(output, "call reaches unavailable device feature") != 3 || strings.Count(output, "import initializes unavailable device feature") != 1 {
		t.Fatalf("unexpected closure findings:\n%s", output)
	}
}

func TestDeviceReachabilityRuntimeSymbolWrappers(t *testing.T) {
	root := checkFixture(t, `package game
import (
	"example.com/game/dependency"
	_ "example.com/game/startup"
)
func run() {
	dependency.Clock()
	dependency.Reflection()
	dependency.Finalizer()
	dependency.Control()
	dependency.Spawn()
	dependency.Channel()
	dependency.Select()
	dependency.Recover()
	dependency.Safe()
}
`)
	fixtures := map[string]string{
		"dependency": `package dependency
import ("time"; "reflect"; "runtime")
func Clock() { clock() }
func clock() { f := time.Now; _ = f() }
func Reflection() { f := func() { _ = reflect.MakeSlice(reflect.TypeOf([]int{}), 0, 0) }; f() }
func Finalizer() { runtime.SetFinalizer(new(int), func(*int) {}) }
func Control() { runtime.LockOSThread() }
func Spawn() { f := func() { go func() {}() }; f() }
func Channel() { channel() }
func channel() { _ = make(chan int) }
func Select() { select { default: } }
func Recover() { (recover)() }
func Safe() {
	_ = func() { go func() {}(); _ = make(chan int); select { default: }; recover() }
	d, _ := time.ParseDuration("1s")
	_ = d.String()
	_ = d.Milliseconds()
	_ = reflect.ValueOf(1).Int()
	_ = reflect.TypeOf(1).Kind()
	runtime.GC()
}
`,
		"startup": `package startup
import "example.com/game/dependency"
func init() { dependency.Clock() }
var started = func() int { go func() {}(); return 1 }()
var channel = make(chan int)
func init() { select { default: }; recover() }
`,
	}
	for name, source := range fixtures {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		writeAnalyzerFixture(t, root, name+"/dependency.go", source)
	}
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Patterns: []string{"."}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) {
		t.Fatalf("fixture load errors: %+v", snapshot.Packages)
	}
	want := map[RuleID]string{
		"device-time-runtime":    "time.Now",
		"device-reflect-symbol":  "reflect.MakeSlice",
		"device-finalizer":       "runtime.SetFinalizer",
		"device-runtime-control": "runtime.LockOSThread",
		"device-goroutine":       "go statement",
		"device-channel":         "channel",
		"device-select":          "select statement",
		"device-recover":         "recover()",
	}
	for rule, symbol := range want {
		t.Run(string(rule), func(t *testing.T) {
			result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{rule}})
			if err != nil {
				t.Fatal(err)
			}
			count := 1
			if rule == "device-time-runtime" || rule == "device-goroutine" || rule == "device-channel" || rule == "device-select" || rule == "device-recover" {
				count = 2
			}
			if len(result.Findings) != count {
				t.Fatalf("findings = %+v, want %d", result.Findings, count)
			}
			for _, finding := range result.Findings {
				if finding.RuleID != rule || !strings.Contains(finding.Message, symbol) || strings.Contains(finding.Message, "dependency.Safe") {
					t.Errorf("unexpected finding: %+v", finding)
				}
			}
		})
	}
}

func assertDeviceCheck(t *testing.T, options CheckOptions, arguments []string, wantCode int, contains string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := RunCheck(context.Background(), arguments, &stdout, &stderr, options)
	if wantCode == ExitSuccess {
		if err != nil {
			t.Fatalf("RunCheck(%v) = %v", arguments, err)
		}
	} else {
		var commandError *CommandError
		if !errors.As(err, &commandError) || commandError.Code != wantCode {
			t.Fatalf("RunCheck(%v) = %v, want exit %d", arguments, err, wantCode)
		}
	}
	if !strings.Contains(stdout.String(), contains) {
		t.Fatalf("RunCheck(%v) output missing %q:\n%s\nstderr: %s", arguments, contains, stdout.String(), stderr.String())
	}
}

func writeAnalyzerFixture(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
