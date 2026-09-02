package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeviceCgoOnlyPackageRemainsReadOnly(t *testing.T) {
	root := checkFixture(t, "package game\nimport \"C\"\n")
	// An ignored file using the first candidate name must not be overwritten.
	reserved := "zz_gopdsdk_check_cgo_0.go"
	writeAnalyzerFixture(t, root, reserved, "//go:build reserved_fixture\n\npackage game\n")
	overlay := map[string][]byte{filepath.Join(root, "game.go"): []byte("package game\nimport \"C\"\n")}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: TargetDevice, Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) || len(snapshot.Packages) != 1 {
		t.Fatalf("snapshot: %+v", snapshot.Packages)
	}
	if len(overlay) != 1 || len(snapshot.overlay) != 1 {
		t.Fatalf("virtual sources leaked into overlay: %v, %v", overlay, snapshot.overlay)
	}
	for _, pkg := range snapshot.Packages {
		if len(pkg.Files) != 1 || pkg.Files[0].Path != "game.go" {
			t.Fatalf("unexpected sources: %+v", pkg.Files)
		}
	}
	for _, pkg := range snapshot.Loaded {
		for _, file := range pkg.Syntax {
			if strings.Contains(pkg.Fset.Position(file.Pos()).Filename, "zz_gopdsdk_check_cgo_") {
				t.Fatal("virtual syntax leaked")
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "zz_gopdsdk_check_cgo_") && entry.Name() != reserved {
			t.Fatalf("wrote stub %s", entry.Name())
		}
	}
}
