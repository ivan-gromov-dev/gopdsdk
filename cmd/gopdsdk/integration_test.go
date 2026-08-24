package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIExternalConsumerWorkflow(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binaryName := "gopdsdk"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), binaryName)
	runTestCommand(t, repository, "go", "build", "-buildvcs=false", "-o", binary, "./cmd/gopdsdk")

	project := filepath.Join(t.TempDir(), "external game")
	runTestCommand(t, repository, binary, "init", "--module", "example.com/acceptance", "--author", "CI", "--bundle-id", "com.example.acceptance", project)
	goMod, err := os.ReadFile(filepath.Join(project, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "replace github.com/ivan-gromov-dev/gopdsdk =>") {
		t.Fatalf("checkout acceptance go.mod does not contain a local replace:\n%s", goMod)
	}
	runTestCommand(t, project, "go", "mod", "tidy")
	runTestCommand(t, project, "go", "test", "./...")

	for _, commandName := range []string{"crashlog", "errorlog"} {
		command := exec.Command(binary, commandName, "--sdk", filepath.Join(project, "fake sdk"))
		output, runErr := command.CombinedOutput()
		if runErr == nil || !strings.Contains(string(output), "required file") {
			t.Fatalf("%s routing: error = %v, output = %q; want missing pdutil error", commandName, runErr, output)
		}
	}

	for _, test := range []struct {
		arguments []string
		target    string
	}{
		{arguments: []string{"build", "--dry-run", "--sdk", filepath.Join(project, "fake sdk"), "."}, target: "simulator"},
		{arguments: []string{"build", "device", "--dry-run", "--sdk", filepath.Join(project, "fake sdk"), "."}, target: "device"},
	} {
		output := runTestCommand(t, project, binary, test.arguments...)
		for _, required := range []string{"Build plan", "Target:      " + test.target, "Application: ."} {
			if !strings.Contains(output, required) {
				t.Fatalf("%v output does not contain %q:\n%s", test.arguments, required, output)
			}
		}
	}
}

func runTestCommand(t *testing.T, directory, executable string, arguments ...string) string {
	t.Helper()
	command := exec.Command(executable, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", executable, arguments, err, output)
	}
	return string(output)
}
