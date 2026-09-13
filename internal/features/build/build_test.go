package build

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
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

func TestStructuredBuildResultNormalizesArtifact(t *testing.T) {
	var output bytes.Buffer
	if err := writeStructuredResult(&output, Result{PackageImport: "example.com/game", Output: `C:\work\game.pdx`}); err != nil {
		t.Fatal(err)
	}
	envelope, err := toolingprotocol.DecodeEnvelope(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var result StructuredResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != ResultSchema || result.Target != "simulator" || result.Artifact != "C:/work/game.pdx" {
		t.Fatalf("structured result = %+v", result)
	}
}

func TestRunStructuredFailureAndProgressAreSeparated(t *testing.T) {
	var stdout, stderr bytes.Buffer
	sdkPath := t.TempDir()
	err := Run(t.Context(), []string{"build", "--format", "json", "--progress", "--sdk", sdkPath, "."}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run succeeded with incomplete SDK")
	}
	envelope, decodeErr := toolingprotocol.DecodeEnvelope(stdout.Bytes())
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if envelope.OK || envelope.Failure == nil || envelope.Failure.Category != "build-failed" {
		t.Fatalf("failure envelope = %+v", envelope)
	}
	if !strings.Contains(stderr.String(), `"schema":"gopdsdk-progress/v1"`) || !strings.Contains(stderr.String(), `"stage":"planning"`) {
		t.Fatalf("progress stream = %q", stderr.String())
	}
	if strings.Contains(stdout.String(), sdkPath) {
		t.Fatalf("failure leaked path: %s", stdout.String())
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
