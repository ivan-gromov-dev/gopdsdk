package simprobe

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestContainsExport(t *testing.T) {
	output := `The Export Tables
	[Ordinal/Name Pointer] Table
		[   0] eventHandler
		[   1] unrelated_eventHandler_suffix
`
	if !containsExport(output, "eventHandler") {
		t.Fatal("containsExport did not find exact export")
	}
	if containsExport(output, "Handler") {
		t.Fatal("containsExport accepted a partial export name")
	}
}

func TestCommandErrorIncludesOutput(t *testing.T) {
	err := commandError("build", errProbe, []byte("compiler detail\n"))
	if got, want := err.Error(), "build: probe failure: compiler detail"; got != want {
		t.Fatalf("commandError() = %q, want %q", got, want)
	}
}

func TestRenderProbeApplication(t *testing.T) {
	initMarker := filepath.Join(t.TempDir(), "Temp path", "init.marker")
	updateMarker := filepath.Join(t.TempDir(), "Temp path", "update.marker")
	source := renderProbeApplication(
		"github.com/ivan-gromov-dev/gopdsdk",
		initMarker,
		updateMarker,
	)
	for _, want := range []string{
		`"github.com/ivan-gromov-dev/gopdsdk/playdate"`,
		strconv.Quote(filepath.ToSlash(initMarker)),
		strconv.Quote(filepath.ToSlash(updateMarker)),
		`context.DrawText("Hello from gopdsdk", 16, 16)`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("renderProbeApplication() does not contain %q:\n%s", want, source)
		}
	}
}

func TestWaitForFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marker")
	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = os.WriteFile(path, []byte("ready"), 0o600)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := waitForFile(ctx, path, time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForFileContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := waitForFile(ctx, filepath.Join(t.TempDir(), "missing"), time.Millisecond); err != context.Canceled {
		t.Fatalf("waitForFile() error = %v, want context.Canceled", err)
	}
}

type probeError string

func (e probeError) Error() string { return string(e) }

const errProbe = probeError("probe failure")
