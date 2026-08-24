// Package gomodule locates and renders metadata for repository-local probe modules.
package gomodule

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Path is the canonical gopdsdk module path.
const Path = "github.com/ivan-gromov-dev/gopdsdk"

// Info identifies the active gopdsdk module.
type Info struct {
	Root      string
	Path      string
	GoVersion string
}

// Locate resolves the canonical gopdsdk module selected by the Go command.
func Locate(ctx context.Context) (Info, error) {
	output, err := exec.CommandContext(ctx, "go", "list", "-m", "-json", Path).CombinedOutput()
	if err != nil {
		return Info{}, fmt.Errorf("locate gopdsdk module: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var module struct{ Path, Dir, GoVersion string }
	if err := json.Unmarshal(output, &module); err != nil {
		return Info{}, fmt.Errorf("locate gopdsdk module: decode go list output: %w", err)
	}
	if module.Path != Path || module.Dir == "" || module.GoVersion == "" {
		return Info{}, fmt.Errorf("locate gopdsdk module: incomplete module metadata")
	}
	return Info{Root: filepath.Clean(module.Dir), Path: module.Path, GoVersion: module.GoVersion}, nil
}

// RenderProbe returns a go.mod for a temporary child module.
func RenderProbe(info Info, suffix string) string {
	return fmt.Sprintf("module %s/%s\n\ngo %s\n\nrequire %s v0.0.0\n\nreplace %s => %s\n",
		info.Path, suffix, info.GoVersion, info.Path, info.Path,
		FormatPath(info.Root))
}

// FormatPath returns the canonical go.mod spelling of a local replacement.
func FormatPath(path string) string {
	path = filepath.ToSlash(path)
	if strings.ContainsAny(path, " \t\r\n\"") {
		return strconv.Quote(path)
	}
	return path
}
