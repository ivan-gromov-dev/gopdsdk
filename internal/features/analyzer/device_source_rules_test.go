package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sourceRuleFindings(t *testing.T, root string, rule RuleID) []Finding {
	t.Helper()
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Patterns: []string{"."}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) {
		t.Fatalf("load errors: %+v", snapshot.Packages)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{IDs: []RuleID{rule}})
	if err != nil {
		t.Fatal(err)
	}
	return result.Findings
}

func TestDevicePanicCleanupPaths(t *testing.T) {
	root := checkFixture(t, `package game
func cleanup() {}
func direct() { defer cleanup(); panic("stop") }
func branch(b bool) { if b { defer cleanup() }; panic("stop") }
func loop(n int) { for i := 0; i < n; i++ { defer cleanup() }; panic("stop") }
func normal() { defer cleanup(); return }
func early(b bool) { if b { defer cleanup(); return }; panic("stop") }
func before() { panic("stop"); defer cleanup() }
func closure() { f := func() { defer cleanup() }; _ = f; panic("stop") }
func shadow() { panic := func(string) {}; defer cleanup(); panic("normal call") }
func inner() { f := func() { defer cleanup(); (panic)("stop") }; f() }
`)
	findings := sourceRuleFindings(t, root, "device-panic-cleanup")
	if len(findings) != 4 {
		t.Fatalf("findings = %+v", findings)
	}
	for _, line := range []string{":3:", ":4:", ":5:", ":11:"} {
		found := false
		for _, finding := range findings {
			if strings.Contains(finding.Position, line) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing line %s: %+v", line, findings)
		}
	}
}

func TestDeviceSourceExcludedPlatforms(t *testing.T) {
	for _, test := range []struct {
		name, expression string
		want             bool
	}{
		{"game.go", "gc", true}, {"game.go", "!tinygo", true},
		{"game.go", "gc || tinygo", false}, {"game.go", "linux && arm", false},
		{"game.go", "feature", false}, {"game.go", "feature && gc", true},
		{"game.go", "cgo", false}, {"game.go", "!cgo", false},
		{"game.go", "go1.24 && baremetal", false}, {"game.go", "windows || feature", false},
		{"game_windows.go", "", true}, {"game_darwin.go", "", true}, {"game_linux.go", "", false},
		{"game_amd64.go", "", true}, {"game_linux_arm.go", "", false}, {"game_windows_arm_test.go", "", true},
		{"game_windows_linux.go", "", false}, {"game_unix.go", "", false},
	} {
		t.Run(test.name+test.expression, func(t *testing.T) {
			data := []byte("//go:build " + test.expression + "\n\npackage game\n")
			if got := deviceSourceExcluded(test.name, data); got != test.want {
				t.Fatalf("excluded = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDeviceCompilerDirectives(t *testing.T) {
	root := checkFixture(t, `package game
import _ "unsafe"
//go:linkname stop runtime.Goexit
func stop()
//go:linkname safe runtime.GC
func safe()
//go:noinline
func normal() {}
var text = "//go:linkname ignored runtime.Goexit"
/* //go:linkname ignored2 runtime.Goexit */
`)
	findings := sourceRuleFindings(t, root, "device-compiler-directive")
	if len(findings) != 1 || !strings.Contains(findings[0].Position, ":3:") {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDeviceBuildConstraintCommentGrammar(t *testing.T) {
	for _, test := range []struct {
		source string
		want   bool
	}{
		{"// +build !tinygo\n\npackage game\n", true},
		{"/*\n//go:build !tinygo\n*/\npackage game\n", false},
		{"package game\n//go:build !tinygo\n", false},
		{"// +build linux\n// +build arm\n\npackage game\n", false},
	} {
		if got := deviceSourceExcluded("game.go", []byte(test.source)); got != test.want {
			t.Errorf("excluded(%q) = %v", test.source, got)
		}
	}
}

func TestDeviceAssemblyDeclarations(t *testing.T) {
	root := checkFixture(t, "package game\nfunc Fast()\nfunc Unrelated()\n")
	writeAnalyzerFixture(t, root, "fast.s", "// a Go assembler implementation\nTEXT ·Fast(SB),$0-0\n RET\n/*\nTEXT ·Unrelated(SB),$0-0\n*/\n")
	findings := sourceRuleFindings(t, root, "device-go-assembly")
	if len(findings) != 1 || !strings.Contains(findings[0].Position, ":2:") {
		t.Fatalf("findings = %+v", findings)
	}
	writeAnalyzerFixture(t, root, "fast.s", "//go:build !tinygo\n\nTEXT ·Fast(SB),$0-0\n RET\n")
	if findings := sourceRuleFindings(t, root, "device-go-assembly"); len(findings) != 0 {
		t.Fatalf("host-only assembly: %+v", findings)
	}
}

func TestDeviceGenericChannelArguments(t *testing.T) {
	root := checkFixture(t, `package game
import "example.com/game/dependency"
func id[T any](value T) T { return value }
var a = id(dependency.Stream(nil))
var b = id[dependency.Stream](nil)
var safe = id(1)
`)
	if err := os.MkdirAll(filepath.Join(root, "dependency"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeAnalyzerFixture(t, root, "dependency/stream.go", "package dependency\ntype Stream chan int\n")
	findings := sourceRuleFindings(t, root, "device-channel")
	if len(findings) != 2 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestDeviceNewSourceRulePolicies(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = checkFixture(t, "package game\nfunc cleanup() {}\n")
	writeAnalyzerFixture(t, options.ModuleRoot, "generated.go", "// Code generated by fixture. DO NOT EDIT.\n\npackage game\nfunc generated() { defer cleanup(); panic(1) }\n")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-panic-cleanup"}, ExitSuccess, "No findings.")
	assertDeviceCheck(t, options, []string{"check", "--target", "both", "--rules", "device-panic-cleanup", "--generated", "include"}, ExitFindings, "device-panic-cleanup")
	assertDeviceCheck(t, options, []string{"check", "--target", "simulator", "--rules", "device-panic-cleanup", "--generated", "include"}, ExitSuccess, "No findings.")
	writeAnalyzerFixture(t, options.ModuleRoot, "tagged.go", "//go:build source_fixture\n\npackage game\nfunc tagged() { defer cleanup(); panic(1) }\n")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-panic-cleanup", "--tags", "source_fixture"}, ExitFindings, "tagged.go")
	writeAnalyzerFixture(t, options.ModuleRoot, "game_test.go", "package game\nfunc testOnly() { defer cleanup(); panic(1) }\n")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-panic-cleanup", "--tests"}, ExitFindings, "game_test.go")
	writeAnalyzerFixture(t, options.ModuleRoot, "suppressed.go", "package game\nfunc suppressed() {\ndefer cleanup()\n//gopdsdk:ignore device-panic-cleanup -- terminal failure is intentional\npanic(1)\n}\n")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-panic-cleanup"}, ExitSuccess, "suppressed inline")
	writeAnalyzerFixture(t, options.ModuleRoot, "host.go", "//go:build !tinygo\n\npackage game\n")
	assertDeviceCheck(t, options, []string{"check", "--target", "device", "--rules", "device-build-constraint"}, ExitSuccess, "host-selected source")
}
