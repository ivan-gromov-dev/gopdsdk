// Package main provides the gopdsdk command-line tool.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/analyzer"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/build"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/deviceconnect"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/devicelog"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/deviceprobe"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/doctor"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/initproject"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/simprobe"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/simrun"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var err error
	if len(args) == 0 {
		err = fmt.Errorf("expected a command (try \"gopdsdk doctor\")")
	} else {
		switch args[0] {
		case "build":
			if len(args) > 1 && args[1] == "device" {
				err = deviceprobe.Run(ctx, args, stdout, stderr)
			} else {
				err = build.Run(ctx, args, stdout, stderr)
			}
		case "crashlog", "errorlog":
			err = devicelog.Run(ctx, args, stdout, stderr)
		case "check":
			options, optionsErr := analyzer.DefaultCheckOptions()
			if optionsErr != nil {
				err = &analyzer.CommandError{Code: analyzer.ExitInternal, Err: optionsErr}
			} else {
				err = analyzer.RunCheck(ctx, args, stdout, stderr, options)
			}
		case "lsp":
			options, optionsErr := analyzer.DefaultCheckOptions()
			if optionsErr != nil {
				err = &analyzer.CommandError{Code: analyzer.ExitInternal, Err: optionsErr}
			} else {
				err = analyzer.RunLSP(ctx, args, os.Stdin, stdout, stderr, options)
			}
		case "doctor":
			err = doctor.Run(ctx, args, stdout, stderr, doctor.Options{
				SimulatorProbe: func(ctx context.Context, sdkPath string) error {
					_, probeErr := simprobe.Probe(ctx, simprobe.Config{SDKPath: sdkPath})
					return probeErr
				},
				DeviceProbe: func(ctx context.Context, sdkPath string) error {
					_, probeErr := deviceprobe.Probe(ctx, deviceprobe.Config{SDKPath: sdkPath})
					return probeErr
				},
			})
		case "init":
			err = initproject.Run(ctx, args, stdout, stderr)
		case "probe":
			if len(args) > 1 && args[1] == "connection" {
				err = deviceconnect.Run(ctx, args, stdout, stderr)
			} else if len(args) > 1 && args[1] == "device" {
				err = deviceprobe.Run(ctx, args, stdout, stderr)
			} else {
				err = simprobe.Run(ctx, args, stdout, stderr)
			}
		case "run":
			if len(args) > 1 && args[1] == "device" {
				err = deviceprobe.Run(ctx, args, stdout, stderr)
			} else {
				err = simrun.Run(ctx, args, stdout, stderr, simrun.Options{})
			}
		default:
			err = fmt.Errorf("unknown command %q", args[0])
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "gopdsdk:", err)
		if coded, ok := err.(interface{ ExitCode() int }); ok {
			return coded.ExitCode()
		}
		return 2
	}
	return 0
}
