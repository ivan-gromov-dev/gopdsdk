package build

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/gomodule"
)

func TestRenderGoModForSameModule(t *testing.T) {
	sdkDir := filepath.Join(t.TempDir(), "Work tree", "gopdsdk")
	info := module{Path: sdkModule, Dir: sdkDir, GoVersion: "1.26.5"}
	goMod := renderGoMod(info, info)
	for _, want := range []string{
		"module github.com/ivan-gromov-dev/gopdsdk/build",
		"go 1.26.5",
		"require github.com/ivan-gromov-dev/gopdsdk v0.0.0",
		"replace github.com/ivan-gromov-dev/gopdsdk => " + gomodule.FormatPath(sdkDir),
	} {
		if !strings.Contains(goMod, want) {
			t.Errorf("renderGoMod() does not contain %q:\n%s", want, goMod)
		}
	}
}

func TestRenderGoModForExternalApplication(t *testing.T) {
	sdkDir := filepath.Join(t.TempDir(), "SDK")
	gameDir := filepath.Join(t.TempDir(), "Game")
	goMod := renderGoMod(
		module{Path: sdkModule, Dir: sdkDir, Version: "v0.1.0", GoVersion: "1.26.5"},
		module{Path: "example.com/game", Dir: gameDir, GoVersion: "1.26.5"},
	)
	for _, want := range []string{
		"require github.com/ivan-gromov-dev/gopdsdk v0.1.0",
		"require example.com/game v0.0.0",
		"replace example.com/game => " + gomodule.FormatPath(gameDir),
	} {
		if !strings.Contains(goMod, want) {
			t.Errorf("renderGoMod() does not contain %q:\n%s", want, goMod)
		}
	}
}
