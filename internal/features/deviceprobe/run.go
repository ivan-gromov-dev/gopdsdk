package deviceprobe

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/buildplan"
)

// Run executes the device probe command.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 || args[1] != "device" || (args[0] != "probe" && args[0] != "build" && args[0] != "run") {
		return fmt.Errorf("expected \"gopdsdk probe device\", \"gopdsdk build device\", or \"gopdsdk run device\"")
	}
	runDevice := args[0] == "run"
	buildDevice := args[0] == "build"
	commandName := "gopdsdk " + args[0] + " device"
	flags := flag.NewFlagSet(commandName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	install := flags.Bool("install", false, "install the verified probe package on a connected Playdate")
	artifactsDir := flags.String("artifacts", "", "directory for diagnostic build artifacts")
	output := flags.String("output", "", "output .pdx path")
	force := flags.Bool("force", false, "replace an existing .pdx output")
	dryRun := flags.Bool("dry-run", false, "print the build plan without executing tools")
	memory := flags.String("memory", string(buildplan.DeviceMemoryConservative), "device memory strategy: conservative (default) or none (legacy diagnostic)")
	format := flags.String("format", "text", "output format for probe device: text or json")
	structuredProgress := flags.Bool("progress", false, "write structured build progress events to stderr")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported device probe format %q", *format)
	}
	if *format == "json" && args[0] == "run" {
		return fmt.Errorf("--format json is not yet supported by \"gopdsdk run device\"")
	}
	if *structuredProgress && (*format != "json" || !buildDevice) {
		return fmt.Errorf("--progress requires \"gopdsdk build device --format json\"")
	}
	application := "./examples/hello"
	if buildDevice || runDevice {
		application = "."
	}
	if flags.NArg() == 1 {
		application = flags.Arg(0)
	}
	memoryStrategy := buildplan.DeviceMemoryStrategy(*memory)
	if *dryRun {
		if !buildDevice {
			return fmt.Errorf("--dry-run is supported by \"gopdsdk build device\"")
		}
		plan, err := buildplan.NewDevice(application, *sdkPath, *output, memoryStrategy)
		if err != nil {
			return err
		}
		return buildplan.Write(stdout, plan)
	}
	if *sdkPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			*sdkPath = filepath.Join(home, "Documents", "PlaydateSDK")
		}
	}
	sequence := 0
	var progress func(string)
	if *structuredProgress {
		progress = func(stage string) {
			sequence++
			_ = toolingprotocol.WriteProgress(stderr, toolingprotocol.ProgressEvent{Schema: toolingprotocol.ProgressSchema, Command: "build device", Sequence: sequence, Stage: stage})
		}
	}
	result, err := Probe(ctx, Config{SDKPath: *sdkPath, Application: application, Output: *output, Replace: *force || runDevice, Persist: buildDevice, Install: *install || runDevice, Run: runDevice, ArtifactsDir: *artifactsDir, Memory: memoryStrategy, Progress: progress})
	if err != nil {
		if *format == "json" {
			if buildDevice {
				return writeStructuredBuildFailure(stdout, ctx, err)
			}
			category := "probe-failed"
			if ctx.Err() != nil {
				category = "cancelled"
			}
			if writeErr := toolingprotocol.WriteFailure(stdout, "probe device", category, &toolingprotocol.Remediation{Action: "inspect-device-toolchain"}); writeErr != nil {
				return writeErr
			}
			return &toolingprotocol.SilentError{Err: err}
		}
		return err
	}
	if *format == "json" {
		if buildDevice {
			return writeStructuredBuildResult(stdout, result)
		}
		values := []toolingprotocol.ProbeValue{
			{Name: "compiler", Value: result.GCC}, {Name: "deployment", Value: result.Deploy}, {Name: "elfBytes", Value: result.Metrics.ELF},
			{Name: "execution", Value: result.Run}, {Name: "export", Value: result.Export}, {Name: "format", Value: result.Format},
			{Name: "output", Value: filepath.ToSlash(result.Output)}, {Name: "package", Value: result.Package}, {Name: "pdxBytes", Value: result.Metrics.PDX},
			{Name: "pending", Value: result.Pending}, {Name: "staticRAMBytes", Value: result.Metrics.StaticRAM}, {Name: "tinygo", Value: result.TinyGo},
		}
		return toolingprotocol.WriteResult(stdout, "probe device", toolingprotocol.ProbeResult{Schema: toolingprotocol.ProbeSchema, Probe: "device", Discovered: true, Ready: true, EvidenceLevel: "device-build", Values: values})
	}
	_, err = fmt.Fprintf(stdout, "Device package stage: READY\nTinyGo:              %s\nCompiler:            %s\nELF:                 %s\nStatic RAM:          %d bytes\nELF size:            %d bytes\nPDX size:            %d bytes\nExport:              %s\nPackage:             %s\nOutput:              %s\nDeployment:          %s\nExecution:           %s\nStill unverified:    %s\n",
		result.TinyGo, result.GCC, result.Format, result.Metrics.StaticRAM, result.Metrics.ELF, result.Metrics.PDX, result.Export, result.Package, result.Output, result.Deploy, result.Run, result.Pending)
	return err
}
