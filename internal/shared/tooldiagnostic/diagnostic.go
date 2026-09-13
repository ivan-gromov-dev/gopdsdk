// Package tooldiagnostic extracts safe source coordinates from tool failures.
package tooldiagnostic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Location struct {
	Path         string
	Line, Column int
}
type commandError struct {
	action string
	err    error
	output []byte
}

func (e *commandError) Error() string {
	if s := strings.TrimSpace(string(e.output)); s != "" {
		return fmt.Sprintf("%s: %v: %s", e.action, e.err, s)
	}
	return fmt.Sprintf("%s: %v", e.action, e.err)
}
func (e *commandError) Unwrap() error { return e.err }
func New(action string, err error, output []byte) error {
	return &commandError{action, err, append([]byte(nil), output...)}
}
func Action(err error) string {
	var e *commandError
	if errors.As(err, &e) {
		return e.action
	}
	return ""
}

type locatedError struct {
	err       error
	locations []Location
}

func (e *locatedError) Error() string { return e.err.Error() }
func (e *locatedError) Unwrap() error { return e.err }
func Locations(err error) []Location {
	var e *locatedError
	if errors.As(err, &e) {
		return append([]Location(nil), e.locations...)
	}
	return nil
}

var pattern = regexp.MustCompile(`^(.+):([0-9]+):([0-9]+):(?: |$)`)

func Attach(err error, root string) error {
	var command *commandError
	if !errors.As(err, &command) {
		return err
	}
	root, absErr := filepath.Abs(filepath.Clean(root))
	if absErr != nil {
		return err
	}
	seen := map[Location]bool{}
	var result []Location
	for _, line := range strings.Split(string(command.output), "\n") {
		m := pattern.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		path := filepath.Clean(filepath.FromSlash(m[1]))
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
			continue
		}
		lineNo, lineErr := strconv.Atoi(m[2])
		col, colErr := strconv.Atoi(m[3])
		if lineErr != nil || colErr != nil || lineNo < 1 || col < 1 {
			continue
		}
		loc := Location{filepath.ToSlash(rel), lineNo, col}
		if !seen[loc] {
			seen[loc] = true
			result = append(result, loc)
		}
	}
	if len(result) == 0 {
		return err
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		return result[i].Column < result[j].Column
	})
	return &locatedError{err, result}
}
