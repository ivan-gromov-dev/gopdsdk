// Package simrun builds and launches gopdsdk applications in Playdate Simulator.
package simrun

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/build"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
)

// Options supplies run dependencies for the command boundary.
type Options struct {
	Build  func(context.Context, build.Config) (build.Result, error)
	Launch func(string, string) (int, error)
}

// Run executes the run command.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, options Options) error {
	flags := flag.NewFlagSet("gopdsdk run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	output := flags.String("output", "", "output .pdx path")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("expected at most one application package")
	}
	application := "."
	if flags.NArg() == 1 {
		application = flags.Arg(0)
	}
	buildApplication := options.Build
	if buildApplication == nil {
		buildApplication = build.Simulator
	}
	launchSimulator := options.Launch
	if launchSimulator == nil {
		launchSimulator = Launch
	}
	result, err := buildApplication(ctx, build.Config{
		SDKPath: *sdkPath,
		Package: application,
		Output:  *output,
		Replace: true,
	})
	if err != nil {
		return err
	}
	pid, err := launchSimulator(*sdkPath, result.Output)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Built %s\nOutput: %s\nSimulator PID: %d\n", result.PackageImport, result.Output, pid)
	return err
}

// Launch starts Playdate Simulator and leaves it running.
func Launch(sdkPath, pdxPath string) (int, error) {
	policy, err := hostpolicy.For(runtime.GOOS)
	if err != nil {
		return 0, err
	}
	var simulator string
	for _, relative := range policy.SimulatorCandidates {
		candidate := filepath.Join(filepath.Clean(sdkPath), relative)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			simulator = candidate
			break
		}
	}
	if simulator == "" {
		return 0, fmt.Errorf("Playdate Simulator is unavailable under %s", filepath.Join(filepath.Clean(sdkPath), "bin"))
	}
	pdxPath, err = filepath.Abs(filepath.Clean(pdxPath))
	if err != nil {
		return 0, fmt.Errorf("resolve .pdx path: %w", err)
	}
	if info, err := os.Stat(pdxPath); err != nil || !info.IsDir() {
		return 0, fmt.Errorf("required .pdx directory %s is unavailable", pdxPath)
	}
	command := exec.Command(simulator, pdxPath)
	command.Dir = filepath.Dir(pdxPath)
	if err := command.Start(); err != nil {
		return 0, fmt.Errorf("launch Simulator: %w", err)
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return 0, fmt.Errorf("release Simulator process: %w", err)
	}
	return pid, nil
}
