package build

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/buildplan"
)

// Run executes the build command.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gopdsdk build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	output := flags.String("output", "", "output .pdx path")
	replace := flags.Bool("force", false, "replace an existing .pdx output")
	dryRun := flags.Bool("dry-run", false, "print the build plan without executing tools")
	format := flags.String("format", "text", "output format: text or json")
	structuredProgress := flags.Bool("progress", false, "write structured progress events to stderr")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("expected at most one application package")
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported build format %q", *format)
	}
	if *structuredProgress && *format != "json" {
		return fmt.Errorf("--progress requires --format json")
	}
	if *dryRun && *format == "json" {
		return fmt.Errorf("--format json is not supported with --dry-run")
	}
	application := "."
	if flags.NArg() == 1 {
		application = flags.Arg(0)
	}
	if *dryRun {
		plan, err := buildplan.New(buildplan.Simulator, application, *sdkPath, *output)
		if err != nil {
			return err
		}
		return buildplan.Write(stdout, plan)
	}
	sequence := 0
	var progress func(string)
	if *structuredProgress {
		progress = func(stage string) {
			sequence++
			_ = toolingprotocol.WriteProgress(stderr, toolingprotocol.ProgressEvent{Schema: toolingprotocol.ProgressSchema, Command: "build", Sequence: sequence, Stage: stage})
		}
	}
	result, err := Simulator(ctx, Config{SDKPath: *sdkPath, Package: application, Output: *output, Replace: *replace, Progress: progress})
	if err != nil {
		if *format == "json" {
			return writeStructuredFailure(stdout, ctx, err)
		}
		return err
	}
	if *format == "json" {
		return writeStructuredResult(stdout, result)
	}
	_, err = fmt.Fprintf(stdout, "Built %s\nOutput: %s\n", result.PackageImport, result.Output)
	return err
}
