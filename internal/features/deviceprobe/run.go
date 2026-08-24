package deviceprobe

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
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
	result, err := Probe(ctx, Config{SDKPath: *sdkPath, Application: application, Output: *output, Replace: *force || runDevice, Persist: buildDevice, Install: *install || runDevice, Run: runDevice, ArtifactsDir: *artifactsDir, Memory: memoryStrategy})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Device package stage: READY\nTinyGo:              %s\nCompiler:            %s\nELF:                 %s\nStatic RAM:          %d bytes\nELF size:            %d bytes\nPDX size:            %d bytes\nExport:              %s\nPackage:             %s\nOutput:              %s\nDeployment:          %s\nExecution:           %s\nStill unverified:    %s\n",
		result.TinyGo, result.GCC, result.Format, result.Metrics.StaticRAM, result.Metrics.ELF, result.Metrics.PDX, result.Export, result.Package, result.Output, result.Deploy, result.Run, result.Pending)
	return err
}
