package initproject

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCreateWritesBuildableProjectContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "My Game")
	sdkDir := filepath.Join(t.TempDir(), "Work tree", "gopdsdk")
	result, err := Create(Config{Path: path, SDKDir: sdkDir, GoVersion: "1.26.5"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Module != "example.com/my-game" {
		t.Fatalf("module = %q", result.Module)
	}
	wants := map[string][]string{
		"go.mod":  {"module example.com/my-game", "replace github.com/ivan-gromov-dev/gopdsdk => " + strconv.Quote(filepath.ToSlash(sdkDir))},
		"game.go": {"// Package game", "func New() playdate.Game", "context.DrawText"},
		"pdxinfo": {"name=My Game", "author=Your Name", "bundleID=com.example.my-game", "buildNumber=1"},
	}
	for name, fragments := range wants {
		contents, readErr := os.ReadFile(filepath.Join(path, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(contents), fragment) {
				t.Errorf("%s does not contain %q:\n%s", name, fragment, contents)
			}
		}
	}
}

func TestCreateWritesPublishedModuleContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Versioned Game")
	if _, err := Create(Config{Path: path, SDKVersion: "v0.1.0", GoVersion: "1.26.5"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(path, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(contents)
	if !strings.Contains(got, "require github.com/ivan-gromov-dev/gopdsdk v0.1.0") {
		t.Fatalf("go.mod = %q", got)
	}
	if strings.Contains(got, "replace ") {
		t.Fatalf("published go.mod contains replace: %q", got)
	}
}

func TestCreateDoesNotModifyExistingPath(t *testing.T) {
	path := t.TempDir()
	marker := filepath.Join(path, "keep")
	if err := os.WriteFile(marker, []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Create(Config{Path: path, SDKDir: "sdk", GoVersion: "1.26.5"})
	if err == nil {
		t.Fatal("Create() error = nil")
	}
	if contents, readErr := os.ReadFile(marker); readErr != nil || string(contents) != "user data" {
		t.Fatalf("existing data changed: %q, %v", contents, readErr)
	}
}

func TestProjectSlug(t *testing.T) {
	if got, want := projectSlug(" My  First_Game! "), "my-first-game"; got != want {
		t.Fatalf("projectSlug() = %q, want %q", got, want)
	}
}
