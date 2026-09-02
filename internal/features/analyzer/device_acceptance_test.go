package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This opt-in test performs real device compilation and packaging. It never
// connects to a Playdate, deploys a package, or reads device logs.
func TestAnalyzerDeviceBuildAcceptance(t *testing.T) {
	if os.Getenv("GOPDSDK_ANALYZER_DEVICE_ACCEPTANCE") != "1" {
		t.Skip("set GOPDSDK_ANALYZER_DEVICE_ACCEPTANCE=1 for real SDK/device-build comparisons")
	}
	sdk := os.Getenv("PLAYDATE_SDK_PATH")
	if sdk == "" {
		t.Fatal("PLAYDATE_SDK_PATH is required for device acceptance")
	}
	evidence := t.TempDir()
	if parent := os.Getenv("GOPDSDK_ANALYZER_EVIDENCE_DIR"); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		var err error
		evidence, err = os.MkdirTemp(parent, "device-acceptance-")
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("evidence: %s", evidence)
	repository := repositoryRoot(t)
	binary := filepath.Join(evidence, "gopdsdk")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", binary, "./cmd/gopdsdk")
	build.Dir = repository
	build.Env = environmentWith("GOWORK", "off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	for _, fixture := range []struct {
		name, imports, declarations, update, rejection string
		rule                                           RuleID
	}{
		{name: "safe", imports: "\"time\"", update: "defer ctx.Clear(); ctx.DrawText(time.Second.String(), 0, 0)"},
		{name: "assembly", declarations: "func Fast()", update: "Fast()", rule: "device-go-assembly", rejection: "undefined reference to"},
		// TinyGo can inline Call's panic stub, leaving no forbidden ELF symbol.
		// Successful packaging here is deliberately not a safe-runtime claim.
		{name: "reflection-trap", imports: "\"reflect\"", update: "reflect.ValueOf(func() { ctx.Clear() }).Call(nil)", rule: "device-reflect-symbol"},
		{name: "reflection", imports: "\"reflect\"", update: "value := reflect.MakeSlice(reflect.TypeOf([]int{}), 1+int(ctx.CurrentTimeMilliseconds()%3), 4); ctx.DrawText(\"slice\", int(value.Index(0).Int()), 0)", rule: "device-reflect-symbol", rejection: "unsupported TinyGo runtime symbols remain: reflect.MakeSlice"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			project := filepath.Join(evidence, fixture.name)
			if err := os.MkdirAll(project, 0o755); err != nil {
				t.Fatal(err)
			}
			writeAnalyzerFixture(t, project, "go.mod", fmt.Sprintf("module example.com/deviceacceptance/%s\n\ngo 1.26.5\n\nrequire github.com/ivan-gromov-dev/gopdsdk v0.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %q\n", fixture.name, filepath.ToSlash(repository)))
			writeAnalyzerFixture(t, project, "game.go", fmt.Sprintf("package game\nimport (\"github.com/ivan-gromov-dev/gopdsdk/playdate\"; %s)\ntype game struct{}\nfunc New() playdate.Game { return game{} }\nfunc (game) Init(playdate.Context) error { return nil }\n%s\nfunc (game) Update(ctx playdate.Context) (bool, error) { %s; return true, nil }\n", fixture.imports, fixture.declarations, fixture.update))
			writeAnalyzerFixture(t, project, "pdxinfo", "name=Analyzer acceptance\nauthor=gopdsdk\nbundleID=sdk.gopdsdk.analyzer\nversion=0.0.1\nbuildNumber=1\n")
			if fixture.name == "assembly" {
				writeAnalyzerFixture(t, project, "fast.s", "TEXT ·Fast(SB),$0-0\n RET\n")
			}
			check := exec.CommandContext(ctx, binary, "check", "--target", "device", "--format", "json", ".")
			check.Dir, check.Env = project, environmentWith("GOWORK", "off")
			var stdout, stderr bytes.Buffer
			check.Stdout, check.Stderr = &stdout, &stderr
			err := check.Run()
			wantCode := 0
			if fixture.rule != "" {
				wantCode = ExitFindings
			}
			code := 0
			if err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					t.Fatal(err)
				}
				code = exitError.ExitCode()
			}
			if code != wantCode {
				t.Fatalf("check exit %d, want %d: %s\n%s", code, wantCode, stdout.String(), stderr.String())
			}
			var report Report
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if fixture.rule == "" && len(report.Diagnostics) != 0 {
				t.Fatalf("safe findings: %+v", report.Diagnostics)
			}
			if fixture.rule != "" && (len(report.Diagnostics) != 1 || report.Diagnostics[0].Rule != fixture.rule) {
				t.Fatalf("findings: %+v", report.Diagnostics)
			}
			writeAnalyzerFixture(t, project, "check.json", stdout.String())
			compile := exec.CommandContext(ctx, binary, "build", "device", "--sdk", sdk, "--output", filepath.Join(project, "game.pdx"), "--artifacts", filepath.Join(project, "artifacts"), ".")
			compile.Dir, compile.Env = project, environmentWith("GOWORK", "off")
			output, err := compile.CombinedOutput()
			writeAnalyzerFixture(t, project, "build.log", string(output))
			if fixture.rejection == "" {
				if err != nil {
					t.Fatalf("device build: %v\n%s", err, output)
				}
				if _, err := os.Stat(filepath.Join(project, "game.pdx", "pdex.bin")); err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(string(output), fixture.rejection) {
				t.Fatalf("expected device rejection %q, got %v\n%s", fixture.rejection, err, output)
			}
			t.Logf("device result:\n%s", output)
		})
	}
}
