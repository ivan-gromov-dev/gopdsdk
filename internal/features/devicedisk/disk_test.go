package devicedisk

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseMountPath(t *testing.T) {
	path := "/Volumes/PLAYDATE"
	if runtime.GOOS == "windows" {
		path = `F:\`
	}
	got, err := ParseMountPath("Playdate data disk mounted as " + path + "\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(path) {
		t.Fatalf("ParseMountPath() = %q, want %q", got, filepath.Clean(path))
	}
}

func TestParseMountPathRejectsUntrustedOutput(t *testing.T) {
	for _, output := range []string{"device detected", "Playdate data disk mounted as relative"} {
		if _, err := ParseMountPath(output); err == nil || !strings.Contains(err.Error(), "disk") {
			t.Fatalf("ParseMountPath(%q) error = %v", output, err)
		}
	}
}
