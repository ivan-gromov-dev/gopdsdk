package artifactreplace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryReplacesOnlyAfterStaging(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "game.pdx")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "new"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Directory(context.Background(), source, target, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "new")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "old")); !os.IsNotExist(err) {
		t.Fatalf("old artifact remains: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".gopdsdk-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary siblings remain: %v", matches)
	}
}

func TestDirectoryConflictAndCancellationPreserveTarget(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "game.pdx")
	for _, path := range []string{source, target} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "new"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Directory(context.Background(), source, target, false); err == nil {
		t.Fatal("conflict accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Directory(ctx, source, target, true); err == nil {
		t.Fatal("cancellation ignored")
	}
	if data, err := os.ReadFile(filepath.Join(target, "old")); err != nil || string(data) != "old" {
		t.Fatalf("target changed: %q, %v", data, err)
	}
}
