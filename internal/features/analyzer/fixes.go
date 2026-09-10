package analyzer

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fixMode string

const (
	fixNone    fixMode = "none"
	fixPreview fixMode = "preview"
	fixApply   fixMode = "apply"
)

type pendingEdit struct {
	path       string
	start, end int
	newText    string
}

type fixedFile struct {
	path     string
	absolute string
	before   []byte
	after    []byte
}

func prepareFixes(moduleRoot string, report Report) ([]fixedFile, error) {
	byPath := make(map[string][]Edit)
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Suppression != nil || len(diagnostic.Edits) == 0 {
			continue
		}
		// Suggested-fix groups are alternatives. Fix-all deterministically chooses
		// the first group emitted by the rule.
		for _, edit := range diagnostic.Edits[0].Edits {
			if !validModuleRelativePath(edit.Range.Path) {
				return nil, fmt.Errorf("fix for %s has unsafe path %q", diagnostic.Rule, edit.Range.Path)
			}
			byPath[edit.Range.Path] = append(byPath[edit.Range.Path], edit)
		}
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	root, err := filepath.Abs(moduleRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve fix root: %w", err)
	}
	result := make([]fixedFile, 0, len(paths))
	for _, path := range paths {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		info, err := os.Lstat(absolute)
		if err != nil {
			return nil, fmt.Errorf("inspect fix target %q: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !pathWithinRoot(root, absolute) {
			return nil, fmt.Errorf("fix target %q is not a regular file within the module", path)
		}
		contents, err := os.ReadFile(absolute)
		if err != nil {
			return nil, fmt.Errorf("read fix target %q: %w", path, err)
		}
		pending := make([]pendingEdit, 0, len(byPath[path]))
		for _, edit := range byPath[path] {
			start, err := pointOffset(contents, edit.Range.Start)
			if err != nil {
				return nil, fmt.Errorf("fix %q start: %w", path, err)
			}
			end, err := pointOffset(contents, edit.Range.End)
			if err != nil || end < start {
				return nil, fmt.Errorf("fix %q has invalid range", path)
			}
			pending = append(pending, pendingEdit{path: path, start: start, end: end, newText: edit.NewText})
		}
		sort.Slice(pending, func(i, j int) bool {
			if pending[i].start != pending[j].start {
				return pending[i].start < pending[j].start
			}
			if pending[i].end != pending[j].end {
				return pending[i].end < pending[j].end
			}
			return pending[i].newText < pending[j].newText
		})
		unique := pending[:0]
		for _, edit := range pending {
			if len(unique) != 0 {
				prior := unique[len(unique)-1]
				if edit.start == prior.start && edit.end == prior.end && edit.newText == prior.newText {
					continue
				}
				if edit.start < prior.end || edit.start == prior.start {
					return nil, fmt.Errorf("conflicting fixes in %q", path)
				}
			}
			unique = append(unique, edit)
		}
		var output bytes.Buffer
		cursor := 0
		for _, edit := range unique {
			output.Write(contents[cursor:edit.start])
			output.WriteString(edit.newText)
			cursor = edit.end
		}
		output.Write(contents[cursor:])
		after := output.Bytes()
		if strings.HasSuffix(strings.ToLower(path), ".go") {
			after, err = format.Source(after)
			if err != nil {
				return nil, fmt.Errorf("format fixed %q: %w", path, err)
			}
		}
		if !bytes.Equal(contents, after) {
			result = append(result, fixedFile{path: path, absolute: absolute, before: contents, after: after})
		}
	}
	return result, nil
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func pointOffset(contents []byte, point Point) (int, error) {
	if point.Line < 1 || point.Column < 1 {
		return 0, fmt.Errorf("invalid point %d:%d", point.Line, point.Column)
	}
	line, offset := 1, 0
	for line < point.Line && offset < len(contents) {
		if contents[offset] == '\n' {
			line++
		}
		offset++
	}
	if line != point.Line {
		return 0, fmt.Errorf("line %d is outside file", point.Line)
	}
	offset += point.Column - 1
	if offset > len(contents) || bytes.Contains(contents[offset-(point.Column-1):offset], []byte{'\n'}) {
		return 0, fmt.Errorf("column %d is outside line %d", point.Column, point.Line)
	}
	return offset, nil
}

func validateFixedFiles(ctx context.Context, moduleRoot string, files []fixedFile, patterns, tags []string, tests bool, targets []Target) error {
	overlay := make(map[string][]byte, len(files))
	for _, file := range files {
		overlay[file.absolute] = file.after
	}
	for _, target := range targets {
		snapshot, err := LoadPackages(ctx, LoadConfig{ModuleRoot: moduleRoot, Patterns: patterns, BuildTags: tags, Tests: tests, Target: target, Overlay: overlay})
		if err != nil {
			return err
		}
		if snapshotHasLoadErrors(snapshot) {
			return formatLoadErrors(snapshotLoadErrors(snapshot))
		}
	}
	return nil
}

func snapshotLoadErrors(snapshot Snapshot) []LoadError {
	var result []LoadError
	for _, pkg := range snapshot.Packages {
		result = append(result, pkg.Errors...)
	}
	return result
}

func writeFixedFiles(files []fixedFile) error {
	for _, file := range files {
		current, err := os.ReadFile(file.absolute)
		if err != nil {
			return fmt.Errorf("re-read fix target %q: %w", file.path, err)
		}
		if !bytes.Equal(current, file.before) {
			return fmt.Errorf("fix target %q changed after analysis", file.path)
		}
	}
	for _, file := range files {
		info, err := os.Stat(file.absolute)
		if err != nil {
			return fmt.Errorf("inspect fix target %q: %w", file.path, err)
		}
		if err := os.WriteFile(file.absolute, file.after, info.Mode()); err != nil {
			return fmt.Errorf("write fixed %q: %w", file.path, err)
		}
	}
	return nil
}

func writeFixPreview(out interface{ Write([]byte) (int, error) }, files []fixedFile) error {
	for _, file := range files {
		if _, err := fmt.Fprintf(out, "--- a/%s\n+++ b/%s\n", file.path, file.path); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "@@ complete file @@\n-%s\n+%s\n", strings.TrimSuffix(string(file.before), "\n"), strings.TrimSuffix(string(file.after), "\n")); err != nil {
			return err
		}
	}
	return nil
}
