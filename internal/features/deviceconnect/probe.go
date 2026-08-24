// Package deviceconnect safely probes Playdate USB connectivity through pdutil.
package deviceconnect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
)

// ErrNoDevice indicates that pdutil could not detect a connected Playdate.
var ErrNoDevice = errors.New("no Playdate device detected")

// Config identifies the official Playdate SDK containing pdutil.
type Config struct {
	SDKPath string
}

// Result records a successful, read-only connectivity probe.
type Result struct {
	Tool   string
	Status string
}

// Probe asks pdutil for help, which opens the device but performs no action.
func Probe(ctx context.Context, config Config) (Result, error) {
	if config.SDKPath == "" {
		return Result{}, fmt.Errorf("Playdate SDK path is required")
	}
	sdkPath, err := filepath.Abs(filepath.Clean(config.SDKPath))
	if err != nil {
		return Result{}, fmt.Errorf("resolve Playdate SDK path: %w", err)
	}
	policy, err := hostpolicy.For(runtime.GOOS)
	if err != nil {
		return Result{}, err
	}
	pdutil := filepath.Join(sdkPath, "bin", policy.PDUtilName)
	if info, statErr := os.Stat(pdutil); statErr != nil || info.IsDir() {
		return Result{}, fmt.Errorf("required file %s is unavailable", pdutil)
	}

	command := exec.CommandContext(ctx, pdutil, "--help")
	output, runErr := command.CombinedOutput()
	return classify(pdutil, string(output), runErr)
}

func classify(tool, output string, runErr error) (Result, error) {
	detail := strings.TrimSpace(output)
	if connection := detectedConnection(detail); connection != "" {
		return Result{Tool: tool, Status: connection + "; no device action was performed"}, nil
	}
	if strings.Contains(strings.ToLower(detail), "no playdate device detected") {
		return Result{}, fmt.Errorf("%w; connect and unlock the Playdate over USB, then retry", ErrNoDevice)
	}
	if runErr != nil {
		if detail == "" {
			return Result{}, fmt.Errorf("probe Playdate connection: %w", runErr)
		}
		return Result{}, fmt.Errorf("probe Playdate connection: %w: %s", runErr, detail)
	}
	if !strings.Contains(detail, "Usage: pdutil") {
		return Result{}, fmt.Errorf("probe Playdate connection: pdutil returned an unrecognized response: %s", detail)
	}
	return Result{Tool: tool, Status: "connected; pdutil opened the device without performing an action"}, nil
}

func detectedConnection(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Playdate device detected on ") {
			return line
		}
	}
	return ""
}
