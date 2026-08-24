// Package deviceprobe verifies TinyGo ARM linking and Playdate packaging.
package deviceprobe

import (
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/buildplan"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/gomodule"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/pdxsource"
)

// Result records the verified device toolchain stage.
type Result struct {
	TinyGo  string
	GCC     string
	Format  string
	Export  string
	Package string
	Output  string
	Deploy  string
	Run     string
	Pending string
	Metrics Metrics
}

// Metrics records reproducible linked and packaged device sizes in bytes.
type Metrics struct {
	StaticRAM uint64
	ELF       int64
	PDX       int64
}

// Config identifies the official Playdate SDK used for the link stage.
type Config struct {
	SDKPath      string
	Application  string
	Output       string
	Replace      bool
	Persist      bool
	Install      bool
	Run          bool
	ArtifactsDir string
	Memory       buildplan.DeviceMemoryStrategy
}

// Probe compiles and links a structural Playdate device ELF.
func Probe(ctx context.Context, config Config) (Result, error) {
	if config.SDKPath == "" {
		return Result{}, fmt.Errorf("Playdate SDK path is required")
	}
	module, err := gomodule.Locate(ctx)
	if err != nil {
		return Result{}, err
	}
	if config.Application == "" {
		config.Application = module.Path + "/examples/hello"
	}
	app, err := inspectApplication(ctx, config.Application)
	if err != nil {
		return Result{}, err
	}
	if app.Name == "main" || app.Module == nil {
		return Result{}, fmt.Errorf("inspect application package: %s must be an importable package in a Go module", app.ImportPath)
	}
	pdxInfo, err := os.ReadFile(filepath.Join(app.Dir, "pdxinfo"))
	if err != nil {
		return Result{}, fmt.Errorf("read application pdxinfo: %w", err)
	}
	if config.Persist && config.Output == "" {
		config.Output = filepath.Join("build", app.Name+".pdx")
	}
	sdkPath, err := filepath.Abs(filepath.Clean(config.SDKPath))
	if err != nil {
		return Result{}, fmt.Errorf("resolve Playdate SDK path: %w", err)
	}
	setupSource := filepath.Join(sdkPath, "C_API", "buildsupport", "setup.c")
	linkerScript := filepath.Join(sdkPath, "C_API", "buildsupport", "link_map.ld")
	policy, err := hostpolicy.For(runtime.GOOS)
	if err != nil {
		return Result{}, err
	}
	pdc := filepath.Join(sdkPath, "bin", policy.PDCName)
	for _, path := range []string{filepath.Join(sdkPath, "C_API", "pd_api.h"), setupSource, linkerScript, pdc} {
		if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
			return Result{}, fmt.Errorf("required file %s is unavailable", path)
		}
	}
	tinygo, err := exec.LookPath("tinygo")
	if err != nil {
		return Result{}, fmt.Errorf("find tinygo: %w", err)
	}
	gcc, err := exec.LookPath("arm-none-eabi-gcc")
	if err != nil {
		return Result{}, fmt.Errorf("find arm-none-eabi-gcc: %w", err)
	}
	readelf, err := exec.LookPath("arm-none-eabi-readelf")
	if err != nil {
		return Result{}, fmt.Errorf("find arm-none-eabi-readelf: %w", err)
	}
	nm, err := exec.LookPath("arm-none-eabi-nm")
	if err != nil {
		return Result{}, fmt.Errorf("find arm-none-eabi-nm: %w", err)
	}
	objcopy, err := exec.LookPath("arm-none-eabi-objcopy")
	if err != nil {
		return Result{}, fmt.Errorf("find arm-none-eabi-objcopy: %w", err)
	}
	workDir, err := os.MkdirTemp("", "gopdsdk-device-probe-")
	if err != nil {
		return Result{}, fmt.Errorf("create device probe directory: %w", err)
	}
	elfPath := filepath.Join(workDir, "pdex.elf")
	pdxName := app.Name + ".pdx"
	pdxPath := filepath.Join(workDir, pdxName)
	if config.Memory == "" {
		config.Memory = buildplan.DeviceMemoryConservative
	}
	plan, err := buildplan.NewDevice(config.Application, sdkPath, config.Output, config.Memory)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return Result{}, err
	}
	plan = buildplan.Resolve(plan, map[string]string{"${WORK}": workDir, "${PACKAGE_OUTPUT}": pdxPath})
	plan = buildplan.BindExecutables(plan, map[string]string{
		"tinygo": tinygo, "arm-none-eabi-objcopy": objcopy, "arm-none-eabi-gcc": gcc,
		"arm-none-eabi-readelf": readelf, "arm-none-eabi-nm": nm, plan.Commands[10].Executable: pdc,
	})
	cleanupPaths, err := buildplan.CleanupPaths(plan)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return Result{}, err
	}
	defer cleanupArtifacts(cleanupPaths)
	for _, file := range []struct {
		name     string
		contents string
	}{
		{name: "go.mod", contents: renderDeviceGoMod(module, app)},
		{name: "main.go", contents: renderProbeSource(module.Path, app.ImportPath)},
		{name: "playdate.json", contents: targetSource},
		{name: "adapter.S", contents: adapterSource},
	} {
		if err := os.WriteFile(filepath.Join(workDir, file.name), []byte(file.contents), 0o644); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", file.name, err)
		}
	}
	for index, planned := range plan.Commands[:7] {
		if _, err := runPlannedCommand(ctx, planned); err != nil {
			return Result{}, err
		}
		if index == 2 {
			bootstrap := bootstrapSource
			if config.Memory == buildplan.DeviceMemoryConservative {
				const graphMarker = "uintptr_t bridgeSoundEffectSource"
				graphIndex := strings.Index(bootstrapSource, graphMarker)
				if graphIndex < 0 {
					return Result{}, errors.New("device audio graph bootstrap is missing")
				}
				bootstrap = conservativeBootstrapSource + "\n" + bootstrapSource[graphIndex:]
			}
			if err := os.WriteFile(filepath.Join(workDir, "bootstrap.c"), []byte(bootstrap), 0o644); err != nil {
				return Result{}, fmt.Errorf("write bootstrap.c: %w", err)
			}
		}
	}
	inspectionOutput, err := runPlannedCommand(ctx, plan.Commands[7])
	if err != nil {
		return Result{}, err
	}
	inspection := inspectionOutput
	for _, check := range []struct {
		description string
		required    string
	}{
		{description: "ELF class", required: "Class:                             ELF32"},
		{description: "ELF type", required: "Type:                              EXEC (Executable file)"},
		{description: "machine", required: "Machine:                           ARM"},
		{description: "export", required: "eventHandler"},
		{description: "entry point", required: "eventHandlerShim"},
		{description: "Go event handler", required: "goEventHandler"},
		{description: "Go update handler", required: "goUpdate"},
		{description: "runtime run", required: "runtime.run"},
		{description: "hard-float ABI", required: "Tag_ABI_VFP_args: VFP registers"},
	} {
		if !strings.Contains(inspection, check.required) {
			return Result{}, fmt.Errorf("inspect ARM object: %s was not verified", check.description)
		}
	}
	if config.Memory == buildplan.DeviceMemoryConservative {
		for _, check := range []struct{ description, required string }{
			{description: "bounded TinyGo heap", required: "playdateRuntimeHeap"},
			{description: "heap start", required: "_heap_start"},
			{description: "heap end", required: "_heap_end"},
			{description: "runtime stack boundary", required: "runtime.stackTop"},
			{description: "shadowed Cortex-M system control pointer", required: "playdateRuntimeSCB"},
		} {
			if !strings.Contains(inspection, check.required) {
				return Result{}, fmt.Errorf("inspect ARM object: %s was not verified", check.description)
			}
		}
	}
	for _, forbidden := range []string{"R_ARM_THM_MOVW_AB", "R_ARM_THM_MOVT_AB"} {
		if strings.Contains(inspection, forbidden) {
			return Result{}, fmt.Errorf("inspect ARM object: unsupported Playdate relocation %s remains", forbidden)
		}
	}
	undefinedOutput, err := runPlannedCommand(ctx, plan.Commands[8])
	if err != nil {
		return Result{}, err
	}
	if unresolved := strongUndefinedSymbols(undefinedOutput); len(unresolved) != 0 {
		return Result{}, fmt.Errorf("inspect unresolved ELF symbols: %s", strings.Join(unresolved, ", "))
	}
	symbolOutput, err := runPlannedCommand(ctx, plan.Commands[9])
	if err != nil {
		return Result{}, err
	}
	lowerSymbols := strings.ToLower(symbolOutput)
	for _, forbidden := range []string{"stm32", "initclk", "machine.tim"} {
		if strings.Contains(lowerSymbols, forbidden) {
			return Result{}, fmt.Errorf("inspect linked ELF symbols: board-specific symbol %q remains", forbidden)
		}
	}
	if unsupported := unsupportedRuntimeSymbols(symbolOutput, config.Memory); len(unsupported) != 0 {
		return Result{}, fmt.Errorf("inspect linked ELF symbols: unsupported TinyGo runtime symbols remain: %s", strings.Join(unsupported, ", "))
	}
	if config.Memory == buildplan.DeviceMemoryConservative {
		if err := validateConservativeHeapSymbols(symbolOutput); err != nil {
			return Result{}, fmt.Errorf("inspect conservative GC heap: %w", err)
		}
	}
	sourceDir := filepath.Join(workDir, "Source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create device package source: %w", err)
	}
	if err := pdxsource.Stage(app.Dir, sourceDir); err != nil {
		return Result{}, err
	}
	packagedELFSource := filepath.Join(sourceDir, "pdex.elf")
	if err := copyFile(elfPath, packagedELFSource); err != nil {
		return Result{}, fmt.Errorf("stage device ELF: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "pdxinfo"), pdxInfo, 0o644); err != nil {
		return Result{}, fmt.Errorf("write device pdxinfo: %w", err)
	}
	if _, err := runPlannedCommand(ctx, plan.Commands[10]); err != nil {
		return Result{}, err
	}
	packagedBinary := filepath.Join(pdxPath, "pdex.bin")
	if err := requireNonEmptyFile(packagedBinary); err != nil {
		return Result{}, fmt.Errorf("verify packaged device binary: %w", err)
	}
	if info, statErr := os.Stat(filepath.Join(pdxPath, "pdxinfo")); statErr != nil || info.IsDir() {
		return Result{}, fmt.Errorf("verify packaged pdxinfo: pdc did not create a file")
	}
	metrics, err := measureDeviceArtifact(elfPath, pdxPath)
	if err != nil {
		return Result{}, err
	}
	if config.ArtifactsDir != "" {
		artifactsDir, err := filepath.Abs(filepath.Clean(config.ArtifactsDir))
		if err != nil {
			return Result{}, fmt.Errorf("resolve artifacts directory: %w", err)
		}
		if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("create artifacts directory: %w", err)
		}
		if err := copyFile(elfPath, filepath.Join(artifactsDir, "pdex.elf")); err != nil {
			return Result{}, fmt.Errorf("save diagnostic ELF: %w", err)
		}
	}
	artifactOutput := "temporary package"
	if config.Output != "" {
		outputPath, err := filepath.Abs(filepath.Clean(config.Output))
		if err != nil {
			return Result{}, fmt.Errorf("resolve output path: %w", err)
		}
		if info, statErr := os.Stat(outputPath); statErr == nil {
			if !config.Replace {
				return Result{}, fmt.Errorf("output already exists: %s", outputPath)
			}
			if !info.IsDir() {
				return Result{}, fmt.Errorf("output path is not a directory: %s", outputPath)
			}
			if err := os.RemoveAll(outputPath); err != nil {
				return Result{}, fmt.Errorf("replace output: %w", err)
			}
		} else if !os.IsNotExist(statErr) {
			return Result{}, fmt.Errorf("inspect output path: %w", statErr)
		}
		if err := copyDirectory(pdxPath, outputPath); err != nil {
			return Result{}, fmt.Errorf("write output: %w", err)
		}
		artifactOutput = outputPath
	}
	deployment := "not requested"
	execution := "not requested"
	pending := "device deployment and hardware execution; allocator is non-collecting"
	if config.Memory == buildplan.DeviceMemoryConservative {
		pending = "device deployment, hardware execution, and conservative-GC soak"
	}
	if config.Install || config.Run {
		pdutil := filepath.Join(sdkPath, "bin", policy.PDUtilName)
		if info, statErr := os.Stat(pdutil); statErr != nil || info.IsDir() {
			return Result{}, fmt.Errorf("required file %s is unavailable", pdutil)
		}
		installOutput, err := runCommandOutput(ctx, workDir, "install device probe", pdutil, "install", pdxPath)
		if err != nil {
			return Result{}, err
		}
		deployment = summarizeOutput(installOutput)
		pending = "hardware execution; allocator is non-collecting"
		if config.Memory == buildplan.DeviceMemoryConservative {
			pending = "hardware execution and conservative-GC soak"
		}
		if config.Run {
			runOutput, err := runDeviceProbe(ctx, workDir, pdutil, "/Games/"+pdxName)
			if err != nil {
				return Result{}, err
			}
			execution = summarizeRunOutput(runOutput)
			pending = "allocator is non-collecting"
			if config.Memory == buildplan.DeviceMemoryConservative {
				pending = "conservative-GC soak and memory-growth measurement"
			}
		}
	}
	return Result{
		TinyGo:  firstLine(runVersion(ctx, tinygo, "version")),
		GCC:     firstLine(runVersion(ctx, gcc, "--version")),
		Format:  "ELF32 ARM executable, hard-float",
		Export:  "eventHandler",
		Package: app.ImportPath,
		Output:  artifactOutput,
		Deploy:  deployment,
		Run:     execution,
		Pending: pending,
		Metrics: metrics,
	}, nil
}

func measureDeviceArtifact(elfPath, pdxPath string) (Metrics, error) {
	elfInfo, err := os.Stat(elfPath)
	if err != nil {
		return Metrics{}, fmt.Errorf("measure device ELF: %w", err)
	}
	image, err := elf.Open(elfPath)
	if err != nil {
		return Metrics{}, fmt.Errorf("measure static RAM: %w", err)
	}
	defer image.Close()
	var staticRAM uint64
	for _, section := range image.Sections {
		if section.Flags&elf.SHF_ALLOC != 0 && section.Flags&elf.SHF_WRITE != 0 {
			staticRAM += section.Size
		}
	}
	pdxSize, err := directoryFileSize(pdxPath)
	if err != nil {
		return Metrics{}, fmt.Errorf("measure device PDX: %w", err)
	}
	return Metrics{StaticRAM: staticRAM, ELF: elfInfo.Size(), PDX: pdxSize}, nil
}

func directoryFileSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func cleanupArtifacts(paths []string) {
	for _, path := range paths {
		_ = os.RemoveAll(path)
	}
}

func runDeviceProbe(ctx context.Context, directory, pdutil, devicePath string) (string, error) {
	const attempts = 10
	for attempt := 1; attempt <= attempts; attempt++ {
		output, err := runCommandOutput(ctx, directory, "run device probe", pdutil, "run", devicePath)
		if err == nil {
			return output, nil
		}
		if !strings.Contains(err.Error(), "No Playdate device detected") || attempt == attempts {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("run device probe: %w", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	return "", fmt.Errorf("run device probe: retry limit reached")
}

func copyFile(source, destination string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, contents, 0o644)
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func requireNonEmptyFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s is empty", path)
	}
	return nil
}

func runPlannedCommand(ctx context.Context, planned buildplan.Command) (string, error) {
	return runCommandOutput(ctx, planned.Directory, planned.Purpose, planned.Executable, planned.Args...)
}

func runCommandOutput(ctx context.Context, directory, action, executable string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", commandError(action, err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func summarizeOutput(output string) string {
	if summary := strings.Join(strings.Fields(output), " "); summary != "" {
		return summary
	}
	return "installed by pdutil"
}

func summarizeRunOutput(output string) string {
	if summary := strings.Join(strings.Fields(output), " "); summary != "" {
		return summary
	}
	return "launch command sent by pdutil"
}

func strongUndefinedSymbols(output string) []string {
	var symbols []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[len(fields)-2] == "U" {
			symbols = append(symbols, fields[len(fields)-1])
		}
	}
	return symbols
}

func unsupportedRuntimeSymbols(output string, memory buildplan.DeviceMemoryStrategy) []string {
	var symbols []string
	if memory != buildplan.DeviceMemoryConservative && strings.Contains(output, "runtime.setupDeferFrame") {
		symbols = append(symbols, "runtime.setupDeferFrame")
	}
	for _, forbidden := range []string{
		"runtime._recover",
		"runtime.SetFinalizer",
		"runtime.setFinalizer",
	} {
		if strings.Contains(output, forbidden) {
			symbols = append(symbols, strings.TrimSpace(forbidden))
		}
	}
	symbols = append(symbols, unsupportedReflectionSymbols(output)...)
	for _, forbidden := range []string{"runtime.chanMake", "runtime.chanSend", "runtime.chanRecv", "runtime.chanClose", "runtime.chanSelect"} {
		if strings.Contains(output, forbidden) {
			symbols = append(symbols, "runtime.chan")
			break
		}
	}
	if memory != buildplan.DeviceMemoryConservative && strings.Contains(output, "runtime/interrupt.In") {
		symbols = append(symbols, "runtime/interrupt.In")
	}
	return symbols
}

var supportedReflectionSymbols = []string{
	"reflect.(*rtype).Field", "reflect.(*rtype).Kind", "reflect.(*rtype).Name",
	"reflect.(*rtype).NumField", "reflect.TypeOf", "reflect.ValueOf",
	"reflect.toRawType", "reflect.toStructField", "reflect.toType",
	"reflect.Value.CanInterface", "reflect.Value.CanSet", "reflect.Value.Convert",
	"reflect.Value.Elem", "reflect.Value.Field", "reflect.Value.Index",
	"reflect.Value.Int", "reflect.Value.Interface", "reflect.Value.IsValid",
	"reflect.Value.Kind", "reflect.Value.MapIndex", "reflect.Value.SetInt",
	"reflect.Value.SetMapIndex", "reflect.Value.SetString",
}

func unsupportedReflectionSymbols(output string) []string {
	var unsupported []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		symbol := fields[len(fields)-1]
		if !strings.HasPrefix(symbol, "reflect.") || reflectionSymbolSupported(symbol) || seen[symbol] {
			continue
		}
		seen[symbol] = true
		unsupported = append(unsupported, symbol)
	}
	return unsupported
}

func reflectionSymbolSupported(symbol string) bool {
	for _, supported := range supportedReflectionSymbols {
		if symbol == supported || strings.HasPrefix(symbol, supported+"$") {
			return true
		}
	}
	return false
}

const conservativeHeapSize = uint64(256 * 1024)

func validateConservativeHeapSymbols(output string) error {
	addresses := make(map[string]uint64)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		address, err := strconv.ParseUint(fields[0], 16, 64)
		if err == nil {
			addresses[fields[len(fields)-1]] = address
		}
	}
	required := []string{"_globals_end", "_heap_start", "_heap_end", "__bss_end__", "playdateRuntimeHeap", "playdateRuntimeSCB", "runtime.stackTop", "runtime.runtimePanicAt", "runtime/interrupt.In", "tinygo_scanCurrentStack"}
	for _, name := range required {
		if _, ok := addresses[name]; !ok {
			return fmt.Errorf("required symbol %s is missing", name)
		}
	}
	start := addresses["_heap_start"]
	end := addresses["_heap_end"]
	if start != addresses["_globals_end"] || start != addresses["playdateRuntimeHeap"] {
		return fmt.Errorf("heap start %#x does not match globals end %#x and storage %#x", start, addresses["_globals_end"], addresses["playdateRuntimeHeap"])
	}
	if start%16 != 0 {
		return fmt.Errorf("heap start %#x is not 16-byte aligned", start)
	}
	if end < start || end-start != conservativeHeapSize {
		return fmt.Errorf("heap range %#x..%#x is %d bytes, want %d", start, end, end-start, conservativeHeapSize)
	}
	if end > addresses["__bss_end__"] {
		return fmt.Errorf("heap end %#x exceeds BSS end %#x", end, addresses["__bss_end__"])
	}
	return nil
}

func runVersion(ctx context.Context, executable, argument string) string {
	output, err := exec.CommandContext(ctx, executable, argument).CombinedOutput()
	if err != nil {
		return executable
	}
	return string(output)
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(value), "\n")
	return strings.TrimSpace(line)
}

func commandError(action string, err error, output []byte) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, detail)
}

func renderProbeSource(modulePath, applicationImport string) string {
	return fmt.Sprintf(
		applicationSourceTemplate,
		modulePath+"/internal/features/runtime",
		modulePath+"/playdate",
		applicationImport,
	)
}

type applicationInfo struct {
	ImportPath string
	Name       string
	Dir        string
	Module     *struct{ Path, Dir, GoVersion string }
}

func renderDeviceGoMod(module gomodule.Info, app applicationInfo) string {
	contents := gomodule.RenderProbe(module, "probe/device")
	if app.Module == nil || app.Module.Path == module.Path {
		return contents
	}
	return contents + fmt.Sprintf("\nrequire %s v0.0.0\nreplace %s => %s\n",
		app.Module.Path, app.Module.Path, strconv.Quote(filepath.ToSlash(app.Module.Dir)))
}

func inspectApplication(ctx context.Context, pattern string) (applicationInfo, error) {
	output, err := exec.CommandContext(ctx, "go", "list", "-json", pattern).CombinedOutput()
	if err != nil {
		return applicationInfo{}, commandError("inspect application package", err, output)
	}
	var app applicationInfo
	if err := json.Unmarshal(output, &app); err != nil {
		return applicationInfo{}, fmt.Errorf("inspect application package: decode go list output: %w", err)
	}
	return app, nil
}
