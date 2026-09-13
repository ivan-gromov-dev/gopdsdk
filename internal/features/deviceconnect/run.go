package deviceconnect

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

// Run executes the read-only device connection probe command.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 || args[0] != "probe" || args[1] != "connection" {
		return fmt.Errorf("expected \"gopdsdk probe connection\"")
	}
	flags := flag.NewFlagSet("gopdsdk probe connection", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported connection probe format %q", *format)
	}
	if *sdkPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			*sdkPath = filepath.Join(home, "Documents", "PlaydateSDK")
		}
	}
	result, err := Probe(ctx, Config{SDKPath: *sdkPath})
	if err != nil {
		if *format == "json" {
			category := "probe-failed"
			action := "inspect-usb-configuration"
			if errors.Is(err, ErrNoDevice) {
				category, action = "not-connected", "connect-and-unlock-device"
			} else if errors.Is(err, context.Canceled) {
				category, action = "cancelled", "retry"
			}
			if writeErr := toolingprotocol.WriteFailure(stdout, "probe connection", category, &toolingprotocol.Remediation{Action: action}); writeErr != nil {
				return writeErr
			}
			return &toolingprotocol.SilentError{Err: err}
		}
		return err
	}
	if *format == "json" {
		values := []toolingprotocol.ProbeValue{{Name: "status", Value: result.Status}, {Name: "tool", Value: filepath.ToSlash(result.Tool)}}
		return toolingprotocol.WriteResult(stdout, "probe connection", toolingprotocol.ProbeResult{Schema: toolingprotocol.ProbeSchema, Probe: "connection", Discovered: true, Ready: true, EvidenceLevel: "usb", Values: values})
	}
	_, err = fmt.Fprintf(stdout, "Device connection: READY\npdutil:           %s\nEvidence:         %s\n", result.Tool, result.Status)
	return err
}
