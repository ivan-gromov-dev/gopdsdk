// Package devicelog retrieves diagnostic logs from a connected Playdate.
package devicelog

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

// Kind identifies a diagnostic log stored at the Playdate data-disk root.
type Kind string

const (
	// CrashLog identifies crashlog.txt.
	CrashLog Kind = "crashlog.txt"
	// ErrorLog identifies errorlog.txt.
	ErrorLog Kind = "errorlog.txt"
)

var (
	ErrNoDevice        = errors.New("no Playdate device detected")
	ErrToolUnavailable = errors.New("pdutil is unavailable")
	ErrLogNotFound     = errors.New("Playdate log is unavailable")
	ErrRetrieval       = errors.New("Playdate log retrieval failed")
)

// Read mounts the Playdate data disk and returns the requested diagnostic log.
func Read(ctx context.Context, sdkPath string, kind Kind) ([]byte, string, error) {
	return read(ctx, sdkPath, kind, nil)
}

func read(ctx context.Context, sdkPath string, kind Kind, progress func(string)) ([]byte, string, error) {
	if kind != CrashLog && kind != ErrorLog {
		return nil, "", fmt.Errorf("unsupported Playdate log %q", kind)
	}
	if sdkPath == "" {
		return nil, "", fmt.Errorf("Playdate SDK path is required")
	}
	sdkPath, err := filepath.Abs(filepath.Clean(sdkPath))
	if err != nil {
		return nil, "", fmt.Errorf("resolve Playdate SDK path: %w", err)
	}
	policy, err := hostpolicy.For(runtime.GOOS)
	if err != nil {
		return nil, "", err
	}
	pdutil := filepath.Join(sdkPath, "bin", policy.PDUtilName)
	if info, statErr := os.Stat(pdutil); statErr != nil || info.IsDir() {
		return nil, "", fmt.Errorf("%w: required file %s", ErrToolUnavailable, pdutil)
	}
	logProgress(progress, "connection")
	if mountPath, ok := findMountedPlaydate(); ok {
		logProgress(progress, "retrieval")
		return readLog(mountPath, kind)
	}
	output, err := exec.CommandContext(ctx, pdutil, "datadisk").CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(output)), "no playdate device detected") {
			return nil, "", ErrNoDevice
		}
		return nil, "", commandError(err, output)
	}
	mountPath, err := parseMountPath(string(output))
	if err != nil {
		return nil, "", err
	}
	logProgress(progress, "retrieval")
	return readLog(mountPath, kind)
}

func logProgress(progress func(string), stage string) {
	if progress != nil {
		progress(stage)
	}
}

func readLog(mountPath string, kind Kind) ([]byte, string, error) {
	logPath := filepath.Join(mountPath, string(kind))
	contents, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("%w: %s", ErrLogNotFound, kind)
		}
		return nil, "", fmt.Errorf("%w: read %s: %v", ErrRetrieval, logLabel(kind), err)
	}
	return contents, logPath, nil
}

func findMountedPlaydate() (string, bool) {
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
	for _, kind := range []Kind{CrashLog, ErrorLog} {
		if info, err := os.Stat(filepath.Join(root, string(kind))); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func parseMountPath(output string) (string, error) {
	const marker = "Playdate data disk mounted as "
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if path, ok := strings.CutPrefix(line, marker); ok {
			path = strings.TrimSpace(path)
			if path == "" || !filepath.IsAbs(path) {
				return "", fmt.Errorf("pdutil returned invalid data disk path %q", path)
			}
			return filepath.Clean(path), nil
		}
	}
	return "", fmt.Errorf("%w: pdutil did not report a mounted data disk", ErrRetrieval)
}

func commandError(err error, output []byte) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%w: mount data disk: %v", ErrRetrieval, err)
	}
	return fmt.Errorf("%w: mount data disk: %v: %s", ErrRetrieval, err, detail)
}

func logLabel(kind Kind) string {
	if kind == ErrorLog {
		return "error log"
	}
	return "crash log"
}
