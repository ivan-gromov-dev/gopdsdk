package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type deviceMatrixFixture struct {
	rule                    RuleID
	source, clean, assembly string
}

// The marker is immediately before the rule's primary source line. This
// exercises the real suppression grammar through the production executable.
func deviceMatrixFixtures() []deviceMatrixFixture {
	return []deviceMatrixFixture{
		{rule: "device-goroutine", source: "package game\nfunc f() {\nMARK\ngo func() {}()\n}\n", clean: "package game\nfunc f() { func() {}() }\n"},
		{rule: "device-channel", source: "package game\nMARK\nvar value chan int\n", clean: "package game\nvar value []int\n"},
		{rule: "device-select", source: "package game\nfunc f() {\nMARK\nselect { default: }\n}\n", clean: "package game\nfunc f() { switch { default: } }\n"},
		{rule: "device-time-runtime", source: "package game\nimport \"time\"\nMARK\nvar _ = time.Now\n", clean: "package game\nimport \"time\"\nvar _ = time.Second.String()\n"},
		{rule: "device-fmt", source: "package game\nMARK\nimport \"fmt\"\nvar _ = fmt.Sprint\n", clean: "package game\nimport \"strconv\"\nvar _ = strconv.Itoa(1)\n"},
		{rule: "device-encoding-json", source: "package game\nMARK\nimport \"encoding/json\"\nvar _ = json.Unmarshal\n", clean: "package game\nvar document = []byte(`{}`)\n"},
		{rule: "device-recover", source: "package game\nfunc f() {\nMARK\nrecover()\n}\n", clean: "package game\nfunc f() { recover := func() {}; recover() }\n"},
		{rule: "device-finalizer", source: "package game\nimport \"runtime\"\nMARK\nvar _ = runtime.SetFinalizer\n", clean: "package game\nimport \"runtime\"\nfunc f() { runtime.GC() }\n"},
		{rule: "device-cgo", source: "package game\nMARK\nimport \"C\"\n", clean: "package game\nvar C = 1\n"},
		{rule: "device-reflect-symbol", source: "package game\nimport \"reflect\"\nMARK\nvar _ = reflect.MakeFunc\n", clean: "package game\nimport \"reflect\"\nvar _ = reflect.ValueOf(1).Int()\n"},
		{rule: "device-runtime-control", source: "package game\nimport \"runtime\"\nMARK\nvar _ = runtime.LockOSThread\n", clean: "package game\nimport \"runtime\"\nvar _ = runtime.GC\n"},
		{rule: "device-panic-cleanup", source: "package game\nfunc f() { defer func() {}()\nMARK\npanic(1)\n}\n", clean: "package game\nfunc f() { defer func() {}(); return }\n"},
		{rule: "device-go-assembly", source: "package game\nMARK\nfunc Fast()\n", clean: "package game\nfunc Fast() {}\n", assembly: "TEXT ·Fast(SB),$0-0\n RET\n"},
		{rule: "device-compiler-directive", source: "package game\nimport _ \"unsafe\"\nMARK\n//go:linkname stop runtime.Goexit\nfunc stop()\n", clean: "package game\nimport _ \"unsafe\"\n//go:linkname collect runtime.GC\nfunc collect()\n"},
		{rule: "device-build-constraint", source: "//go:build !tinygo\n\nMARK\npackage game\n", clean: "//go:build gc || tinygo\n\npackage game\n"},
	}
}

func TestExternalDeviceRulePolicyMatrix(t *testing.T) {
	repository := repositoryRoot(t)
	binary := filepath.Join(t.TempDir(), "gopdsdk")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", binary, "./cmd/gopdsdk")
	build.Dir = repository
	build.Env = environmentWith("GOWORK", "off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	fixtures := deviceMatrixFixtures()
	// Adding a device rule must extend the acceptance matrix as well.
	covered := make(map[RuleID]bool)
	positions := make(map[RuleID]Point)
	columns := map[RuleID]int{"device-channel": 11, "device-time-runtime": 9,
		"device-fmt": 8, "device-encoding-json": 8, "device-finalizer": 9,
		"device-cgo": 8, "device-reflect-symbol": 9, "device-runtime-control": 9,
		"device-go-assembly": 6, "device-build-constraint": 9}
	for _, fixture := range fixtures {
		covered[fixture.rule] = true
		column := columns[fixture.rule]
		if column == 0 {
			column = 1
		}
		positions[fixture.rule] = Point{Line: strings.Count(strings.Split(fixture.source, "MARK")[0], "\n") + 2, Column: column}
	}
	for _, registration := range deviceRuleRegistrations() {
		if !covered[registration.RuleID] {
			t.Fatalf("missing matrix fixture for %s", registration.RuleID)
		}
	}
	for _, mode := range []string{"positive", "negative", "generated-exclude", "generated-include", "tags-exclude", "tags-include", "tests-exclude", "tests", "suppressed", "both", "simulator"} {
		t.Run(mode, func(t *testing.T) {
			root := checkFixture(t, "package game\n")
			for _, fixture := range fixtures {
				dir := filepath.Join(root, string(fixture.rule))
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				source := strings.ReplaceAll(fixture.source, "MARK", "")
				name := "fixture.go"
				switch mode {
				case "negative":
					source = fixture.clean
				case "generated-exclude", "generated-include":
					source = "// Code generated by matrix fixture. DO NOT EDIT.\n\n" + source
				case "tags-exclude", "tags-include":
					if strings.HasPrefix(source, "//go:build ") {
						source = strings.Replace(source, "//go:build ", "//go:build matrix_fixture && ", 1)
					} else {
						source = "//go:build matrix_fixture\n\n" + source
					}
				case "tests", "tests-exclude":
					// Go rejects import C in _test.go. Exercise its package's
					// test variant with C in the production compilation unit.
					if fixture.rule != "device-cgo" {
						name = "fixture_test.go"
					}
					writeAnalyzerFixture(t, dir, "base.go", "package game\n")
					writeAnalyzerFixture(t, dir, "variant_test.go", "package game\nfunc variant() {}\n")
				case "suppressed":
					source = strings.ReplaceAll(fixture.source, "MARK", "//gopdsdk:ignore "+string(fixture.rule)+" -- acceptance fixture exception")
				}
				writeAnalyzerFixture(t, dir, name, source)
				if fixture.assembly != "" && mode != "negative" {
					writeAnalyzerFixture(t, dir, "fast.s", fixture.assembly)
				}
			}
			args := []string{"check", "--target", "device", "--format", "json", "--fail-on", "information"}
			switch mode {
			case "generated-include":
				args = append(args, "--generated", "include")
			case "tags-include":
				args = append(args, "--tags", "matrix_fixture")
			case "tests":
				args = append(args, "--tests")
			case "both", "simulator":
				args[2] = mode
			}
			args = append(args, "./...")
			command := exec.CommandContext(ctx, binary, args...)
			command.Dir = root
			command.Env = environmentWith("GOWORK", "off")
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err := command.Run()
			wantFindings := mode != "negative" && mode != "generated-exclude" && mode != "tags-exclude" && mode != "simulator"
			wantCode := 0
			if wantFindings && mode != "suppressed" {
				wantCode = ExitFindings
			}
			code := 0
			if err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					t.Fatalf("execute: %v", err)
				}
				code = exitError.ExitCode()
			}
			if code != wantCode {
				t.Fatalf("exit %d, want %d\n%s\n%s", code, wantCode, stdout.String(), stderr.String())
			}
			var report Report
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("decode report: %v\n%s", err, stdout.String())
			}
			if report.Schema != ProtocolSchema {
				t.Fatalf("schema: %q", report.Schema)
			}
			seen := make(map[RuleID]bool)
			for _, diagnostic := range report.Diagnostics {
				if !covered[diagnostic.Rule] || diagnostic.Target != TargetDevice {
					t.Errorf("unexpected diagnostic: %+v", diagnostic)
				}
				if !strings.HasPrefix(diagnostic.Primary.Path, string(diagnostic.Rule)+"/") {
					t.Errorf("cross-rule fixture finding: %+v", diagnostic)
				}
				if mode == "positive" && (diagnostic.Primary.Start != positions[diagnostic.Rule] || seen[diagnostic.Rule]) {
					t.Errorf("unstable or duplicate primary position: %+v, want %+v", diagnostic, positions[diagnostic.Rule])
				}
				if (diagnostic.Suppression != nil) != (mode == "suppressed") {
					t.Errorf("suppression: %+v", diagnostic)
				}
				seen[diagnostic.Rule] = true
			}
			for _, fixture := range fixtures {
				wantRule := wantFindings && (mode != "tests-exclude" || fixture.rule == "device-cgo")
				if seen[fixture.rule] != wantRule {
					t.Errorf("rule %s present=%v, want=%v", fixture.rule, seen[fixture.rule], wantRule)
				}
			}
			if mode == "positive" {
				baseline := Baseline{Schema: BaselineSchema}
				for _, diagnostic := range report.Diagnostics {
					baseline.Entries = append(baseline.Entries, BaselineEntry{Rule: diagnostic.Rule, Target: diagnostic.Target,
						Path: diagnostic.Primary.Path, Line: diagnostic.Primary.Start.Line, Column: diagnostic.Primary.Start.Column,
						Message: diagnostic.Message, Reason: "accepted fixture debt"})
				}
				data, err := json.Marshal(baseline)
				if err != nil {
					t.Fatal(err)
				}
				writeAnalyzerFixture(t, root, "baseline.json", string(data))
				baselineArgs := append(append([]string(nil), args[:len(args)-1]...), "--baseline", "baseline.json", "./...")
				command := exec.CommandContext(ctx, binary, baselineArgs...)
				command.Dir, command.Env = root, environmentWith("GOWORK", "off")
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("baseline CLI: %v\n%s", err, output)
				}
				var suppressed Report
				if err := json.Unmarshal(output, &suppressed); err != nil {
					t.Fatal(err)
				}
				if len(suppressed.Diagnostics) != len(report.Diagnostics) {
					t.Fatalf("baseline lost diagnostics: %+v", suppressed)
				}
				for index, diagnostic := range suppressed.Diagnostics {
					original := report.Diagnostics[index]
					if diagnostic.Rule != original.Rule || diagnostic.Primary != original.Primary || diagnostic.Message != original.Message || diagnostic.Suppression == nil || diagnostic.Suppression.Kind != "baseline" {
						t.Errorf("baseline changed identity or failed to suppress: %+v", diagnostic)
					}
				}
			}
		})
	}
}
