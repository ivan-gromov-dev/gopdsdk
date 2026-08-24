package build

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

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
	if *dryRun {
		plan, err := buildplan.New(buildplan.Simulator, application, *sdkPath, *output)
		if err != nil {
			return err
		}
		return buildplan.Write(stdout, plan)
	}
	result, err := Simulator(ctx, Config{SDKPath: *sdkPath, Package: application, Output: *output, Replace: *replace})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Built %s\nOutput: %s\n", result.PackageImport, result.Output)
	return err
}
