// Package artifactreplace commits fully staged artifact directories with rollback.
package artifactreplace

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Directory copies source into a sibling staging directory and then commits it
// to target. An existing target is restored if the final rename fails.
func Directory(ctx context.Context, source, target string, replace bool) error {
	source, err := filepath.Abs(filepath.Clean(source))
	if err != nil {
		return fmt.Errorf("resolve artifact source: %w", err)
	}
	target, err = filepath.Abs(filepath.Clean(target))
	if err != nil || filepath.Dir(target) == target {
		return fmt.Errorf("invalid artifact target %q", target)
	}
	if info, statErr := os.Stat(source); statErr != nil || !info.IsDir() {
		return fmt.Errorf("artifact source is not a directory")
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create artifact parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".gopdsdk-stage-")
	if err != nil {
		return fmt.Errorf("create artifact staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := copyDirectory(ctx, source, stage); err != nil {
		return fmt.Errorf("stage artifact: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("stage artifact: %w", err)
	}
	info, statErr := os.Stat(target)
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect artifact target: %w", statErr)
	}
	if statErr == nil {
		if !replace {
			return fmt.Errorf("output already exists: %s", target)
		}
		if !info.IsDir() {
			return fmt.Errorf("output path is not a directory: %s", target)
		}
		backup, backupErr := reserveSibling(parent, ".gopdsdk-backup-")
		if backupErr != nil {
			return backupErr
		}
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("backup existing artifact: %w", err)
		}
		if err := os.Rename(stage, target); err != nil {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return fmt.Errorf("commit artifact: %v; restore previous artifact: %w", err, restoreErr)
			}
			return fmt.Errorf("commit artifact: %w", err)
		}
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove artifact backup: %w", err)
		}
		return nil
	}
	if err := os.Rename(stage, target); err != nil {
		return fmt.Errorf("commit artifact: %w", err)
	}
	return nil
}

func reserveSibling(parent, pattern string) (string, error) {
	path, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		return "", fmt.Errorf("reserve artifact backup: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("reserve artifact backup: %w", err)
	}
	return path, nil
}

func copyDirectory(ctx context.Context, source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.Mkdir(destination, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported: %s", relative)
		}
		return copyFile(path, destination)
	})
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		input.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	inputErr := input.Close()
	outputErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if inputErr != nil {
		return inputErr
	}
	return outputErr
}
