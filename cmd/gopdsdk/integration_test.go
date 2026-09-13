package main

import (
	"errors"
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

	checkJSON := runTestCommand(t, project, binary, "check", "--target", "shared", "--format", "json", "./...")
	if !strings.Contains(checkJSON, `"schema": "gopdsdk-check/v1"`) || !strings.Contains(checkJSON, `"diagnostics": []`) {
		t.Fatalf("clean check JSON is incomplete:\n%s", checkJSON)
	}
	capabilitiesJSON := runTestCommand(t, project, binary, "capabilities")
	for _, required := range []string{`"schema": "gopdsdk-tooling-result/v1"`, `"command": "capabilities"`, `"schema": "gopdsdk-tooling-capabilities/v1"`} {
		if !strings.Contains(capabilitiesJSON, required) {
			t.Fatalf("capabilities JSON does not contain %q:\n%s", required, capabilitiesJSON)
		}
	}
	doctorJSON := runTestCommand(t, project, binary, "doctor", "--format", "json", "--sdk", filepath.Join(project, "fake sdk"))
	for _, required := range []string{`"command": "doctor"`, `"schema": "gopdsdk-doctor/v1"`, `"failureCategory": "not-found"`} {
		if !strings.Contains(doctorJSON, required) {
			t.Fatalf("doctor JSON does not contain %q:\n%s", required, doctorJSON)
		}
	}
	for _, arguments := range [][]string{
		{"probe", "simulator", "--format", "json", "--sdk", filepath.Join(project, "secret simulator sdk")},
		{"probe", "device", "--format", "json", "--sdk", filepath.Join(project, "secret device sdk")},
		{"probe", "connection", "--format", "json", "--sdk", filepath.Join(project, "secret usb sdk")},
	} {
		command := exec.Command(binary, arguments...)
		command.Dir = project
		command.Env = append(os.Environ(), "GOWORK=off")
		output, runErr := command.CombinedOutput()
		var exitError *exec.ExitError
		if !errors.As(runErr, &exitError) || exitError.ExitCode() != 2 || !strings.Contains(string(output), `"schema": "gopdsdk-tooling-result/v1"`) || !strings.Contains(string(output), `"ok": false`) {
			t.Fatalf("%v structured failure: error = %v, output = %s", arguments, runErr, output)
		}
		if strings.Contains(string(output), "secret") {
			t.Fatalf("%v leaked input path: %s", arguments, output)
		}
	}
	command := exec.Command(binary, "build", "--format", "json", "--progress", "--sdk", filepath.Join(project, "secret build sdk"), ".")
	command.Dir = project
	command.Env = append(os.Environ(), "GOWORK=off")
	buildOutput, buildErr := command.CombinedOutput()
	var buildExitError *exec.ExitError
	if !errors.As(buildErr, &buildExitError) || buildExitError.ExitCode() != 2 || !strings.Contains(string(buildOutput), `"schema":"gopdsdk-progress/v1"`) || !strings.Contains(string(buildOutput), `"command": "build"`) {
		t.Fatalf("structured build failure: error = %v, output = %s", buildErr, buildOutput)
	}
	if strings.Contains(string(buildOutput), "secret build sdk") {
		t.Fatalf("structured build failure leaked input path: %s", buildOutput)
	}
	assertExitCode(t, project, binary, 2, "check", "--format", "xml")
	configPath := filepath.Join(project, ".gopdsdk-check.json")
	if err := os.WriteFile(configPath, []byte(`{"schema":"gopdsdk-check-config/v1","unknown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, project, binary, 2, "check")
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	brokenPath := filepath.Join(project, "broken.go")
	if err := os.WriteFile(brokenPath, []byte("package game\nfunc broken(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, project, binary, 3, "check", "--target", "shared", "./...")
	if err := os.Remove(brokenPath); err != nil {
		t.Fatal(err)
	}

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

func assertExitCode(t *testing.T, directory, executable string, want int, arguments ...string) {
	t.Helper()
	command := exec.Command(executable, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != want {
		t.Fatalf("%s %v: exit = %v, want %d\n%s", executable, arguments, err, want, output)
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
