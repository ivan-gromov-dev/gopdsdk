package simrun

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/build"
	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
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

func TestRunStructuredResultAndProgress(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"run", "--format", "json", "--progress", "--sdk", "C:/PlaydateSDK"}, &stdout, &stderr, Options{
		Build: func(_ context.Context, config build.Config) (build.Result, error) {
			for _, stage := range []string{"planning", "compilation", "packaging", "cleanup"} {
				config.Progress(stage)
			}
			return build.Result{PackageImport: "example.com/game", Output: `C:\work\game.pdx`}, nil
		},
		Launch: func(string, string) (int, error) { return 42, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := toolingprotocol.DecodeEnvelope(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var result StructuredResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != ResultSchema || result.Artifact != "C:/work/game.pdx" || result.PID != 42 {
		t.Fatalf("result = %+v", result)
	}
	progress := stderr.String()
	for sequence, stage := range []string{"planning", "compilation", "packaging", "cleanup", "launch"} {
		want := `"sequence":` + strconv.Itoa(sequence+1) + `,"stage":"` + stage + `"`
		if !strings.Contains(progress, want) {
			t.Fatalf("progress does not contain %q: %s", want, progress)
		}
	}
}

func TestRunStructuredLaunchFailureIsRedacted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"run", "--format", "json"}, &stdout, &stderr, Options{
		Build: func(context.Context, build.Config) (build.Result, error) {
			return build.Result{PackageImport: "example.com/game", Output: "game.pdx"}, nil
		},
		Launch: func(string, string) (int, error) { return 0, errRun("secret launch path") },
	})
	if err == nil {
		t.Fatal("Run succeeded")
	}
	envelope, decodeErr := toolingprotocol.DecodeEnvelope(stdout.Bytes())
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if envelope.Failure == nil || envelope.Failure.Category != "launch-failed" || strings.Contains(stdout.String(), "secret") || stderr.Len() != 0 {
		t.Fatalf("stdout = %s, stderr = %s", stdout.String(), stderr.String())
	}
}

type errRun string

func (e errRun) Error() string { return string(e) }
