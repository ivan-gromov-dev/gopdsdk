package simrun

import (
	"bytes"
	"context"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/build"
)

func TestRunBuildsWithReplacementAndLaunches(t *testing.T) {
	var received build.Config
	var launchedSDK, launchedPDX string
	var stdout bytes.Buffer
	err := Run(t.Context(), []string{"run", "--sdk", "C:/PlaydateSDK", "--output", "game.pdx", "./game"}, &stdout, &bytes.Buffer{}, Options{
		Build: func(_ context.Context, config build.Config) (build.Result, error) {
			received = config
			return build.Result{PackageImport: "example.com/game", Output: "C:/work/game.pdx"}, nil
		},
		Launch: func(sdkPath, pdxPath string) (int, error) {
			launchedSDK, launchedPDX = sdkPath, pdxPath
			return 42, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.Package != "./game" || received.Output != "game.pdx" || !received.Replace {
		t.Fatalf("build config = %+v", received)
	}
	if launchedSDK != "C:/PlaydateSDK" || launchedPDX != "C:/work/game.pdx" {
		t.Fatalf("launch args = %q, %q", launchedSDK, launchedPDX)
	}
	if got, want := stdout.String(), "Built example.com/game\nOutput: C:/work/game.pdx\nSimulator PID: 42\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunStopsAfterBuildError(t *testing.T) {
	buildErr := errRun("build failed")
	launched := false
	err := Run(t.Context(), []string{"run"}, &bytes.Buffer{}, &bytes.Buffer{}, Options{
		Build: func(context.Context, build.Config) (build.Result, error) {
			return build.Result{}, buildErr
		},
		Launch: func(string, string) (int, error) {
			launched = true
			return 0, nil
		},
	})
	if err != buildErr || launched {
		t.Fatalf("Run() error/launched = %v/%v, want build error/false", err, launched)
	}
}

type errRun string

func (e errRun) Error() string { return string(e) }
