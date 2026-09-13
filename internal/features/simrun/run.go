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
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/build"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
)

// Options supplies run dependencies for the command boundary.
type Options struct {
	Build  func(context.Context, build.Config) (build.Result, error)
	Launch func(string, string) (int, error)
}

// ResultSchema identifies a successful structured Simulator run.
const ResultSchema = "gopdsdk-run/v1"

type StructuredResult struct {
	Schema   string `json:"schema"`
	Target   string `json:"target"`
	Package  string `json:"package"`
	Artifact string `json:"artifact"`
	PID      int    `json:"pid"`
}

// Run executes the run command.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, options Options) error {
	flags := flag.NewFlagSet("gopdsdk run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	output := flags.String("output", "", "output .pdx path")
	format := flags.String("format", "text", "output format: text or json")
	structuredProgress := flags.Bool("progress", false, "write structured progress events to stderr")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("expected at most one application package")
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported run format %q", *format)
	}
	if *structuredProgress && *format != "json" {
		return fmt.Errorf("--progress requires --format json")
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
	sequence := 0
	var progress func(string)
	if *structuredProgress {
		progress = func(stage string) {
			sequence++
			_ = toolingprotocol.WriteProgress(stderr, toolingprotocol.ProgressEvent{Schema: toolingprotocol.ProgressSchema, Command: "run", Sequence: sequence, Stage: stage})
		}
	}
	result, err := buildApplication(ctx, build.Config{
		SDKPath:  *sdkPath,
		Package:  application,
		Output:   *output,
		Replace:  true,
		Progress: progress,
	})
	if err != nil {
		if *format == "json" {
			return writeStructuredFailure(stdout, ctx, err, "build-failed", "inspect-build-configuration")
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		if *format == "json" {
			return writeStructuredFailure(stdout, ctx, err, "cancelled", "retry")
		}
		return err
	}
	if progress != nil {
		progress("launch")
	}
	pid, err := launchSimulator(*sdkPath, result.Output)
	if err != nil {
		if *format == "json" {
			return writeStructuredFailure(stdout, ctx, err, "launch-failed", "inspect-simulator-installation")
		}
		return err
	}
	if *format == "json" {
		artifact := filepath.ToSlash(strings.ReplaceAll(result.Output, `\`, "/"))
		return toolingprotocol.WriteResult(stdout, "run", StructuredResult{Schema: ResultSchema, Target: "simulator", Package: result.PackageImport, Artifact: artifact, PID: pid})
	}
	_, err = fmt.Fprintf(stdout, "Built %s\nOutput: %s\nSimulator PID: %d\n", result.PackageImport, result.Output, pid)
	return err
}

func writeStructuredFailure(out io.Writer, ctx context.Context, err error, category, action string) error {
	if ctx.Err() != nil {
		category, action = "cancelled", "retry"
	}
	if writeErr := toolingprotocol.WriteFailure(out, "run", category, &toolingprotocol.Remediation{Action: action}); writeErr != nil {
		return writeErr
	}
	return &toolingprotocol.SilentError{Err: err}
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
