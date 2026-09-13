package simprobe

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

// Run executes the simulator probe command.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 || args[0] != "probe" || args[1] != "simulator" {
		return fmt.Errorf("expected \"gopdsdk probe simulator\"")
	}

	flags := flag.NewFlagSet("gopdsdk probe simulator", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	runSimulator := flags.Bool("run", false, "launch Simulator and verify kEventInit")
	timeout := flags.Duration("timeout", 15*time.Second, "maximum time to wait for Simulator initialization")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported simulator probe format %q", *format)
	}
	if *sdkPath == "" {
		return fmt.Errorf("playdate SDK path is required; set PLAYDATE_SDK_PATH or pass --sdk")
	}

	result, err := Probe(ctx, Config{SDKPath: *sdkPath, RunSimulator: *runSimulator, Timeout: *timeout})
	if err != nil {
		if *format == "json" {
			return structuredFailure(stdout, "probe simulator", probeFailureCategory(err), "inspect-simulator-toolchain")
		}
		return err
	}
	if *format == "json" {
		values := []toolingprotocol.ProbeValue{{Name: "compiler", Value: result.Compiler}, {Name: "event", Value: result.Event}, {Name: "export", Value: result.Export}, {Name: "package", Value: result.Package}, {Name: "sdkVersion", Value: result.SDKVersion}}
		return toolingprotocol.WriteResult(stdout, "probe simulator", toolingprotocol.ProbeResult{Schema: toolingprotocol.ProbeSchema, Probe: "simulator", Discovered: true, Ready: true, EvidenceLevel: "sdk-integration", Values: values})
	}
	_, err = fmt.Fprintf(stdout, "Simulator probe: READY\nSDK:             %s\nCompiler:        %s\nExport:          %s\nPackage:         %s\n",
		result.SDKVersion, result.Compiler, result.Export, result.Package)
	if err == nil && result.Event != "" {
		_, err = fmt.Fprintf(stdout, "Simulator event: %s\n", result.Event)
	}
	return err
}

func probeFailureCategory(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "cancelled"
	}
	return "probe-failed"
}

func structuredFailure(out io.Writer, command, category, action string) error {
	err := fmt.Errorf("%s", category)
	if writeErr := toolingprotocol.WriteFailure(out, command, category, &toolingprotocol.Remediation{Action: action}); writeErr != nil {
		return writeErr
	}
	return &toolingprotocol.SilentError{Err: err}
}
