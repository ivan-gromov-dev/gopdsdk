package devicelog

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

// Run executes the crashlog or errorlog command.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected \"gopdsdk crashlog\" or \"gopdsdk errorlog\"")
	}
	kind, ok := commandKind(args[0])
	if !ok {
		return fmt.Errorf("expected \"gopdsdk crashlog\" or \"gopdsdk errorlog\"")
	}
	flags := flag.NewFlagSet("gopdsdk "+args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	format := flags.String("format", "text", "output format: text or json")
	structuredProgress := flags.Bool("progress", false, "write structured progress events to stderr")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported device log format %q", *format)
	}
	if *structuredProgress && *format != "json" {
		return fmt.Errorf("--progress requires --format json")
	}
	if *sdkPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			*sdkPath = filepath.Join(home, "Documents", "PlaydateSDK")
		}
	}
	sequence := 0
	var progress func(string)
	if *structuredProgress {
		progress = func(stage string) {
			sequence++
			_ = toolingprotocol.WriteProgress(stderr, toolingprotocol.ProgressEvent{Schema: toolingprotocol.ProgressSchema, Command: args[0], Sequence: sequence, Stage: stage})
		}
	}
	contents, path, err := read(ctx, *sdkPath, kind, progress)
	if err != nil {
		if *format == "json" {
			return writeStructuredFailure(stdout, ctx, args[0], err)
		}
		return err
	}
	if *format == "json" {
		return writeStructuredResult(stdout, args[0], kind, path, contents)
	}
	if _, err := fmt.Fprintf(stderr, "%s: %s\n", displayLabel(kind), path); err != nil {
		return err
	}
	_, err = stdout.Write(contents)
	return err
}

func writeStructuredFailure(out io.Writer, ctx context.Context, command string, err error) error {
	category, action := "retrieval-failed", "retry-log-retrieval"
	switch {
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(ctx.Err(), context.DeadlineExceeded):
		category, action = "cancelled", "retry"
	case errors.Is(err, ErrNoDevice):
		category, action = "not-connected", "connect-and-unlock-device"
	case errors.Is(err, ErrToolUnavailable):
		category, action = "tool-not-found", "configure-sdk-path"
	case errors.Is(err, ErrLogNotFound):
		category, action = "log-not-found", "reproduce-and-retry"
	}
	if writeErr := toolingprotocol.WriteFailure(out, command, category, &toolingprotocol.Remediation{Action: action}); writeErr != nil {
		return writeErr
	}
	return &toolingprotocol.SilentError{Err: err}
}

func commandKind(command string) (Kind, bool) {
	switch command {
	case "crashlog":
		return CrashLog, true
	case "errorlog":
		return ErrorLog, true
	default:
		return "", false
	}
}

func displayLabel(kind Kind) string {
	if kind == ErrorLog {
		return "Error log"
	}
	return "Crash log"
}
