package analyzer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExternalCheckDriverFindingFailureAndCancellation(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	name := "checkdriver"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	driver := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-buildvcs=false", "-o", driver, "./internal/features/analyzer/testdata/checkdriver")
	build.Dir = repository
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build checkdriver: %v\n%s", err, output)
	}
	project := checkFixture(t, "package game\nfunc bad() {}\n")
	for _, test := range []struct {
		mode   string
		code   int
		output string
	}{
		{"finding", ExitFindings, "external synthetic finding"},
		{"failure", ExitInternal, "external synthetic analyzer failure"},
		{"cancel", ExitCanceled, "context canceled"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			command := exec.Command(driver, "check", "--target", "device", "--format", "json", "./...")
			command.Dir = project
			command.Env = append(os.Environ(), "GOWORK=off", "GOPDSDK_CHECKDRIVER_MODE="+test.mode)
			output, err := command.CombinedOutput()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != test.code {
				t.Fatalf("exit = %v, want %d\n%s", err, test.code, output)
			}
			if !strings.Contains(string(output), test.output) {
				t.Fatalf("output missing %q:\n%s", test.output, output)
			}
		})
	}
}
