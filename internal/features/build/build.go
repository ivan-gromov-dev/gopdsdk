// Package build compiles gopdsdk applications into Playdate artifacts.
package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/runtime/simabi"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/buildplan"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/gomodule"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/pdxsource"
)

const sdkModule = "github.com/ivan-gromov-dev/gopdsdk"

// Config identifies a Simulator application build.
type Config struct {
	SDKPath string
	Package string
	Output  string
	Replace bool
}

// Result describes the produced Simulator artifact.
type Result struct {
	PackageImport string
	Output        string
}

type module struct {
	Path      string
	Dir       string
	Version   string
	GoVersion string
}

type packageInfo struct {
	ImportPath string
	Name       string
	Dir        string
	Module     *module
}

// Simulator builds an importable Go package into a Playdate Simulator .pdx.
func Simulator(ctx context.Context, config Config) (Result, error) {
	policy, err := hostpolicy.For(runtime.GOOS)
	if err != nil {
		return Result{}, err
	}
	if config.SDKPath == "" {
		return Result{}, fmt.Errorf("Playdate SDK path is required")
	}
	if config.Package == "" {
		config.Package = "."
	}

	app, err := inspectPackage(ctx, config.Package)
	if err != nil {
		return Result{}, err
	}
	if app.Name == "main" {
		return Result{}, fmt.Errorf("inspect application package: %s must be importable, not package main", app.ImportPath)
	}
	if app.Module == nil {
		return Result{}, fmt.Errorf("inspect application package: %s is not in a Go module", app.ImportPath)
	}
	pdxInfo, err := loadPDXInfo(filepath.Join(app.Dir, "pdxinfo"))
	if err != nil {
		return Result{}, err
	}
	sdk, err := inspectModule(ctx, sdkModule)
	if err != nil {
		return Result{}, err
	}

	sdkPath, err := filepath.Abs(filepath.Clean(config.SDKPath))
	if err != nil {
		return Result{}, fmt.Errorf("resolve Playdate SDK path: %w", err)
	}
	pdc := filepath.Join(sdkPath, "bin", policy.PDCName)
	apiHeader := filepath.Join(sdkPath, "C_API", "pd_api.h")
	for _, path := range []string{pdc, apiHeader} {
		if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
			return Result{}, fmt.Errorf("required file %s is unavailable", path)
		}
	}
	compiler, err := lookPathAny(policy.CompilerCandidates)
	if err != nil {
		return Result{}, err
	}

	output := config.Output
	if output == "" {
		output = filepath.Join("build", app.Name+".pdx")
	}
	output, err = filepath.Abs(filepath.Clean(output))
	if err != nil {
		return Result{}, fmt.Errorf("resolve output path: %w", err)
	}
	outputExists := false
	if info, statErr := os.Stat(output); statErr == nil {
		if !config.Replace {
			return Result{}, fmt.Errorf("output already exists: %s", output)
		}
		if !info.IsDir() {
			return Result{}, fmt.Errorf("output path is not a directory: %s", output)
		}
		outputExists = true
	} else if !os.IsNotExist(statErr) {
		return Result{}, fmt.Errorf("inspect output path: %w", statErr)
	}

	workDir, err := os.MkdirTemp("", "gopdsdk-build-")
	if err != nil {
		return Result{}, fmt.Errorf("create build directory: %w", err)
	}
	temporaryPDX := filepath.Join(workDir, "Application.pdx")
	plan, err := buildplan.New(buildplan.Simulator, config.Package, sdkPath, output)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return Result{}, err
	}
	plan = buildplan.Resolve(plan, map[string]string{
		"${WORK}": workDir, "${HOST_LIBRARY}": policy.LibraryExtension, "${CC}": compiler,
		"${PDC}": pdc, "${PACKAGE_OUTPUT}": temporaryPDX,
	})
	cleanupPaths, err := buildplan.CleanupPaths(plan)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return Result{}, err
	}
	defer cleanupArtifacts(cleanupPaths)
	sourceDir := filepath.Join(workDir, "Source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create Source directory: %w", err)
	}
	if err := pdxsource.Stage(app.Dir, sourceDir); err != nil {
		return Result{}, err
	}

	sources, err := simabi.Render(simabi.Config{
		APIHeader:         apiHeader,
		RuntimeImport:     sdkModule + "/internal/features/runtime",
		PlaydateImport:    sdkModule + "/playdate",
		ApplicationImport: app.ImportPath,
	})
	if err != nil {
		return Result{}, err
	}
	files := []struct {
		name     string
		contents string
	}{
		{name: "main.go", contents: sources.Go},
		{name: "bridge.c", contents: sources.C},
		{name: "go.mod", contents: renderGoMod(sdk, *app.Module)},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(workDir, file.name), []byte(file.contents), 0o644); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", file.name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "pdxinfo"), pdxInfo, 0o644); err != nil {
		return Result{}, fmt.Errorf("write pdxinfo: %w", err)
	}

	for index, planned := range plan.Commands {
		if err := executePlannedCommand(ctx, planned); err != nil {
			return Result{}, err
		}
		if index == 0 {
			_ = os.Remove(filepath.Join(sourceDir, "pdex.h"))
		}
	}
	if outputExists {
		if err := os.RemoveAll(output); err != nil {
			return Result{}, fmt.Errorf("replace output: %w", err)
		}
	}
	if err := copyDirectory(temporaryPDX, output); err != nil {
		_ = os.RemoveAll(output)
		return Result{}, fmt.Errorf("write output: %w", err)
	}
	return Result{PackageImport: app.ImportPath, Output: output}, nil
}

func lookPathAny(candidates []string) (string, error) {
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("find host C compiler (tried %s)", strings.Join(candidates, ", "))
}

func cleanupArtifacts(paths []string) {
	for _, path := range paths {
		_ = os.RemoveAll(path)
	}
}

func executePlannedCommand(ctx context.Context, planned buildplan.Command) error {
	command := exec.CommandContext(ctx, planned.Executable, planned.Args...)
	command.Dir = planned.Directory
	command.Env = append(os.Environ(), planned.Environment...)
	if output, err := command.CombinedOutput(); err != nil {
		return commandError(planned.Purpose, err, output)
	}
	return nil
}

func inspectPackage(ctx context.Context, pattern string) (packageInfo, error) {
	output, err := exec.CommandContext(ctx, "go", "list", "-json", pattern).CombinedOutput()
	if err != nil {
		return packageInfo{}, commandError("inspect application package", err, output)
	}
	var info packageInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return packageInfo{}, fmt.Errorf("inspect application package: decode go list output: %w", err)
	}
	return info, nil
}

func inspectModule(ctx context.Context, path string) (module, error) {
	output, err := exec.CommandContext(ctx, "go", "list", "-m", "-json", path).CombinedOutput()
	if err != nil {
		return module{}, commandError("inspect gopdsdk module", err, output)
	}
	var info module
	if err := json.Unmarshal(output, &info); err != nil {
		return module{}, fmt.Errorf("inspect gopdsdk module: decode go list output: %w", err)
	}
	return info, nil
}

func renderGoMod(sdk, app module) string {
	goVersion := sdk.GoVersion
	if goVersion == "" {
		goVersion = app.GoVersion
	}
	sdkVersion := sdk.Version
	if sdkVersion == "" {
		sdkVersion = "v0.0.0"
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "module %s/build\n\ngo %s\n\nrequire %s %s\n", sdkModule, goVersion, sdkModule, sdkVersion)
	if app.Path != sdk.Path {
		fmt.Fprintf(&builder, "require %s v0.0.0\n", app.Path)
	}
	fmt.Fprintf(&builder, "\nreplace %s => %s\n", sdkModule, gomodule.FormatPath(sdk.Dir))
	if app.Path != sdk.Path {
		fmt.Fprintf(&builder, "replace %s => %s\n", app.Path, gomodule.FormatPath(app.Dir))
	}
	return builder.String()
}

func commandError(action string, err error, output []byte) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, detail)
}

func copyDirectory(source, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		return copyFile(path, destination)
	})
}

func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
