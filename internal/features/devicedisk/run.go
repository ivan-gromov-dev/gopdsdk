package devicedisk

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

const ResultSchema = "gopdsdk-device-disk/v1"

type StructuredResult struct {
	Schema    string `json:"schema"`
	Mode      string `json:"mode"`
	MountPath string `json:"mountPath,omitempty"`
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 || args[0] != "device" || args[1] != "disk" || (args[2] != "mount" && args[2] != "unmount") {
		return fmt.Errorf("expected \"gopdsdk device disk mount|unmount\"")
	}
	command := "device disk " + args[2]
	flags := flag.NewFlagSet("gopdsdk "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	sdkPath := flags.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "path to the Playdate SDK")
	format := flags.String("format", "text", "output format: text or json")
	progress := flags.Bool("progress", false, "write structured progress events to stderr")
	if err := flags.Parse(args[3:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported device disk format %q", *format)
	}
	if *progress && *format != "json" {
		return fmt.Errorf("--progress requires --format json")
	}
	if *sdkPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			*sdkPath = filepath.Join(home, "Documents", "PlaydateSDK")
		}
	}
	stage := "unmount"
	if args[2] == "mount" {
		stage = "mount"
	}
	if *progress {
		_ = toolingprotocol.WriteProgress(stderr, toolingprotocol.ProgressEvent{Schema: toolingprotocol.ProgressSchema, Command: command, Sequence: 1, Stage: stage})
	}
	var result Result
	var err error
	if args[2] == "mount" {
		result, err = Mount(ctx, *sdkPath)
	} else {
		result, err = Unmount(ctx, *sdkPath)
	}
	if err != nil {
		if *format == "json" {
			category, action := "disk-mode-failed", "retry"
			if errors.Is(err, ErrNoDevice) {
				category, action = "not-connected", "connect-and-unlock-device"
			}
			if errors.Is(err, ErrNotMounted) {
				category, action = "not-mounted", "mount-device-disk"
			}
			if errors.Is(err, ErrEject) {
				category, action = "eject-failed", "close-device-files-and-retry"
			}
			if errors.Is(err, ErrReconnect) {
				category, action = "reconnect-timeout", "check-device-connection"
			}
			_ = toolingprotocol.WriteFailure(stdout, command, category, &toolingprotocol.Remediation{Action: action})
			return &toolingprotocol.SilentError{Err: err}
		}
		return err
	}
	if *format == "json" {
		return toolingprotocol.WriteResult(stdout, command, StructuredResult{Schema: ResultSchema, Mode: result.Mode, MountPath: filepath.ToSlash(result.MountPath)})
	}
	_, err = fmt.Fprintf(stdout, "Playdate mode: %s\n", result.Mode)
	return err
}
