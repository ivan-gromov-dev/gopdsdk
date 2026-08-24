// Package main provides the gopdsdk command-line tool.
package main

import (
	"context"
	"fmt"
	"os"

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
	args := os.Args[1:]
	var err error
	if len(args) == 0 {
		err = fmt.Errorf("expected a command (try \"gopdsdk doctor\")")
	} else {
		switch args[0] {
		case "build":
			if len(args) > 1 && args[1] == "device" {
				err = deviceprobe.Run(context.Background(), args, os.Stdout, os.Stderr)
			} else {
				err = build.Run(context.Background(), args, os.Stdout, os.Stderr)
			}
		case "crashlog", "errorlog":
			err = devicelog.Run(context.Background(), args, os.Stdout, os.Stderr)
		case "doctor":
			err = doctor.Run(context.Background(), args, os.Stdout, os.Stderr, doctor.Options{
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
			err = initproject.Run(context.Background(), args, os.Stdout, os.Stderr)
		case "probe":
			if len(args) > 1 && args[1] == "connection" {
				err = deviceconnect.Run(context.Background(), args, os.Stdout, os.Stderr)
			} else if len(args) > 1 && args[1] == "device" {
				err = deviceprobe.Run(context.Background(), args, os.Stdout, os.Stderr)
			} else {
				err = simprobe.Run(context.Background(), args, os.Stdout, os.Stderr)
			}
		case "run":
			if len(args) > 1 && args[1] == "device" {
				err = deviceprobe.Run(context.Background(), args, os.Stdout, os.Stderr)
			} else {
				err = simrun.Run(context.Background(), args, os.Stdout, os.Stderr, simrun.Options{})
			}
		default:
			err = fmt.Errorf("unknown command %q", args[0])
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gopdsdk:", err)
		os.Exit(2)
	}
}
