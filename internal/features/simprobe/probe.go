// Package simprobe verifies that Go can produce a loadable Playdate Simulator artifact.
package simprobe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/runtime/simabi"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/gomodule"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
)

// Config identifies the Playdate SDK used by the probe.
type Config struct {
	SDKPath      string
	RunSimulator bool
	Timeout      time.Duration
}

// Result records the independently verified probe stages.
type Result struct {
	SDKVersion string
	Compiler   string
	Export     string
	Package    string
	Event      string
}

// Probe builds, inspects, and packages a minimal Simulator extension.
func Probe(ctx context.Context, config Config) (Result, error) {
	policy, err := hostpolicy.For(runtime.GOOS)
	if err != nil {
		return Result{}, err
	}

	sdkPath, err := filepath.Abs(filepath.Clean(config.SDKPath))
	if err != nil {
		return Result{}, fmt.Errorf("resolve SDK path: %w", err)
	}
	versionBytes, err := os.ReadFile(filepath.Join(sdkPath, "VERSION.txt"))
	if err != nil {
		return Result{}, fmt.Errorf("read SDK version: %w", err)
	}
	pdc := filepath.Join(sdkPath, "bin", policy.PDCName)
	if err := requireFile(pdc); err != nil {
		return Result{}, err
	}
	apiHeader := filepath.Join(sdkPath, "C_API", "pd_api.h")
	if err := requireFile(apiHeader); err != nil {
		return Result{}, err
	}
	compiler, err := findCompiler(policy.CompilerCandidates)
	if err != nil {
		return Result{}, err
	}
	module, err := gomodule.Locate(ctx)
	if err != nil {
		return Result{}, err
	}

	workDir, err := os.MkdirTemp("", "gopdsdk-simulator-probe-")
	if err != nil {
		return Result{}, fmt.Errorf("create probe directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	sourceDir := filepath.Join(workDir, "Source")
	appDir := filepath.Join(workDir, "app")
	initMarkerPath := filepath.Join(workDir, "init.marker")
	updateMarkerPath := filepath.Join(workDir, "update.marker")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create Source directory: %w", err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create probe application directory: %w", err)
	}
	runtimeImport := module.Path + "/internal/features/runtime"
	sources, err := simabi.Render(simabi.Config{
		APIHeader:         apiHeader,
		RuntimeImport:     runtimeImport,
		PlaydateImport:    module.Path + "/playdate",
		ApplicationImport: module.Path + "/probe/app",
	})
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(workDir, "probe.go"), []byte(sources.Go), 0o644); err != nil {
		return Result{}, fmt.Errorf("write probe source: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "bridge.c"), []byte(sources.C), 0o644); err != nil {
		return Result{}, fmt.Errorf("write probe bridge: %w", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.go"), []byte(renderProbeApplication(module.Path, initMarkerPath, updateMarkerPath)), 0o644); err != nil {
		return Result{}, fmt.Errorf("write probe application: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(gomodule.RenderProbe(module, "probe")), 0o644); err != nil {
		return Result{}, fmt.Errorf("write probe module: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "pdxinfo"), []byte(probePDXInfo), 0o644); err != nil {
		return Result{}, fmt.Errorf("write pdxinfo: %w", err)
	}

	libraryName := "pdex." + policy.LibraryExtension
	dllPath := filepath.Join(sourceDir, libraryName)
	build := exec.CommandContext(ctx, "go", "build", "-buildmode=c-shared", "-o", dllPath, ".")
	build.Dir = workDir
	build.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"CC="+compiler,
		"CGO_CFLAGS=-DTARGET_EXTENSION=1 -DTARGET_SIMULATOR=1",
	)
	if output, runErr := build.CombinedOutput(); runErr != nil {
		return Result{}, commandError("build c-shared probe", runErr, output)
	}
	if err := requireFile(dllPath); err != nil {
		return Result{}, err
	}
	_ = os.Remove(filepath.Join(sourceDir, "pdex.h"))

	inspect := exec.CommandContext(ctx, "go", "tool", "nm", dllPath)
	output, runErr := inspect.CombinedOutput()
	if runErr != nil {
		return Result{}, commandError("inspect Simulator exports", runErr, output)
	}
	if !containsExport(string(output), simabi.EventHandlerExport) {
		return Result{}, fmt.Errorf("inspect Simulator exports: %s was not exported", simabi.EventHandlerExport)
	}

	pdxPath := filepath.Join(workDir, "Probe.pdx")
	pack := exec.CommandContext(ctx, pdc, "-sdkpath", sdkPath, sourceDir, pdxPath)
	pack.Dir = workDir
	if output, runErr = pack.CombinedOutput(); runErr != nil {
		return Result{}, commandError("package probe", runErr, output)
	}
	if info, statErr := os.Stat(pdxPath); statErr != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("package probe: pdc did not create %s", pdxPath)
	}
	if err := requireFile(filepath.Join(pdxPath, libraryName)); err != nil {
		return Result{}, fmt.Errorf("package probe: %w", err)
	}

	verifiedEvent := ""
	if config.RunSimulator {
		simulator, err := findSimulator(sdkPath, policy.SimulatorCandidates)
		if err != nil {
			return Result{}, err
		}
		timeout := config.Timeout
		if timeout <= 0 {
			timeout = 15 * time.Second
		}
		if err := launchAndWait(ctx, simulator, pdxPath, initMarkerPath, updateMarkerPath, timeout); err != nil {
			return Result{}, err
		}
		verifiedEvent = "kEventInit + update + drawText"
	}

	return Result{
		SDKVersion: strings.TrimSpace(string(versionBytes)),
		Compiler:   firstLine(runVersion(ctx, compiler)),
		Export:     simabi.EventHandlerExport,
		Package:    libraryName + " in .pdx",
		Event:      verifiedEvent,
	}, nil
}

func findCompiler(candidates []string) (string, error) {
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("find host C compiler (tried %s)", strings.Join(candidates, ", "))
}

func findSimulator(sdkPath string, candidates []string) (string, error) {
	for _, relative := range candidates {
		path := filepath.Join(sdkPath, relative)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("Playdate Simulator is unavailable under %s", filepath.Join(sdkPath, "bin"))
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("required file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("required file %s is a directory", path)
	}
	return nil
}

func containsExport(output, name string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && (fields[len(fields)-1] == name || fields[len(fields)-1] == "_"+name) {
			return true
		}
	}
	return false
}

func commandError(action string, err error, output []byte) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, detail)
}

func runVersion(ctx context.Context, path string) string {
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return path
	}
	return string(output)
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(value), "\n")
	return strings.TrimSpace(line)
}

func renderProbeApplication(modulePath, initMarkerPath, updateMarkerPath string) string {
	return fmt.Sprintf(`package app

import (
	"os"

	%q
)

type game struct{}

func New() playdate.Game { return game{} }

func (game) Init(playdate.Context) error {
	return os.WriteFile(%q, []byte("kEventInit"), 0o600)
}

func (game) Update(context playdate.Context) (bool, error) {
	context.DrawText("Hello from gopdsdk", 16, 16)
	if err := os.WriteFile(%q, []byte("update"), 0o600); err != nil {
		return false, err
	}
	return true, nil
}
`, modulePath+"/playdate", filepath.ToSlash(initMarkerPath), filepath.ToSlash(updateMarkerPath))
}

func launchAndWait(ctx context.Context, simulator, pdxPath, initMarkerPath, updateMarkerPath string, timeout time.Duration) error {
	command := exec.CommandContext(ctx, simulator, pdxPath)
	command.Dir = filepath.Dir(pdxPath)
	if err := command.Start(); err != nil {
		return fmt.Errorf("launch Simulator: %w", err)
	}
	defer func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	}()

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := waitForFile(waitCtx, initMarkerPath, 100*time.Millisecond); err != nil {
		return fmt.Errorf("verify Simulator kEventInit: %w", err)
	}
	if err := waitForFile(waitCtx, updateMarkerPath, 100*time.Millisecond); err != nil {
		return fmt.Errorf("verify Simulator update callback: %w", err)
	}
	return nil
}

func waitForFile(ctx context.Context, path string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

const probePDXInfo = `name=GoPD SDK Simulator Probe
author=gopdsdk
bundleID=sdk.gopdsdk.probe
version=0.0.0
buildNumber=1
`
