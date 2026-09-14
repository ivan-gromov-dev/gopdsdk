// Package devicedisk owns entering and leaving Playdate Data Disk mode.
package devicedisk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/deviceconnect"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
)

var (
	ErrNoDevice        = errors.New("no Playdate device detected")
	ErrNotMounted      = errors.New("Playdate data disk is not mounted")
	ErrToolUnavailable = errors.New("pdutil is unavailable")
	ErrEject           = errors.New("Playdate data disk eject failed")
	ErrReconnect       = errors.New("Playdate did not reconnect after eject")
	execCommand        = exec.CommandContext
	probeConnection    = deviceconnect.Probe
)

type Result struct{ Mode, MountPath string }

func Mount(ctx context.Context, sdkPath string) (Result, error) {
	if root, ok := FindMounted(); ok {
		return Result{Mode: "disk", MountPath: root}, nil
	}
	pdutil, err := toolPath(sdkPath)
	if err != nil {
		return Result{}, err
	}
	output, err := execCommand(ctx, pdutil, "datadisk").CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(output)), "no playdate device detected") {
			return Result{}, ErrNoDevice
		}
		return Result{}, fmt.Errorf("mount data disk: %w: %s", err, strings.TrimSpace(string(output)))
	}
	root, err := ParseMountPath(string(output))
	if err != nil {
		return Result{}, err
	}
	return Result{Mode: "disk", MountPath: root}, nil
}

func Unmount(ctx context.Context, sdkPath string) (Result, error) {
	root, ok := FindMounted()
	if !ok {
		return Result{}, ErrNotMounted
	}
	if err := ejectVolume(ctx, root); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrEject, err)
	}
	reconnectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := probeConnection(reconnectCtx, deviceconnect.Config{SDKPath: sdkPath}); err == nil {
			return Result{Mode: "connected"}, nil
		}
		select {
		case <-reconnectCtx.Done():
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
			return Result{}, ErrReconnect
		case <-ticker.C:
		}
	}
}

func toolPath(sdkPath string) (string, error) {
	if sdkPath == "" {
		return "", fmt.Errorf("Playdate SDK path is required")
	}
	policy, err := hostpolicy.For(runtime.GOOS)
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Clean(sdkPath), "bin", policy.PDUtilName)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", fmt.Errorf("%w: required file %s", ErrToolUnavailable, path)
	}
	return path, nil
}
func FindMounted() (string, bool) {
	for _, root := range mountCandidates() {
		if isPlaydateRoot(root) {
			return root, true
		}
	}
	return "", false
}
func mountCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		roots := make([]string, 0, 24)
		for drive := 'C'; drive <= 'Z'; drive++ {
			roots = append(roots, fmt.Sprintf("%c:\\", drive))
		}
		return roots
	case "darwin":
		roots, _ := filepath.Glob("/Volumes/*")
		return roots
	default:
		var roots []string
		if user := os.Getenv("USER"); user != "" {
			for _, pattern := range []string{filepath.Join("/media", user, "*"), filepath.Join("/run/media", user, "*")} {
				matches, _ := filepath.Glob(pattern)
				roots = append(roots, matches...)
			}
		}
		matches, _ := filepath.Glob("/mnt/*")
		return append(roots, matches...)
	}
}
func isPlaydateRoot(root string) bool {
	for _, directory := range []string{"Data", "Games", "System"} {
		info, err := os.Stat(filepath.Join(root, directory))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}
func ParseMountPath(output string) (string, error) {
	const marker = "Playdate data disk mounted as "
	for _, line := range strings.Split(output, "\n") {
		if path, ok := strings.CutPrefix(strings.TrimSpace(line), marker); ok {
			path = strings.TrimSpace(path)
			if path == "" || !filepath.IsAbs(path) {
				return "", fmt.Errorf("pdutil returned invalid data disk path %q", path)
			}
			return filepath.Clean(path), nil
		}
	}
	return "", fmt.Errorf("pdutil did not report a mounted data disk")
}
