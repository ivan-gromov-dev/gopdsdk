package doctor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/hostpolicy"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/toolchainprofile"
)

type sdkCandidate struct {
	path   string
	source string
}

// Config controls environment discovery.
type Config struct {
	SDKPath string
}

// Inspect assesses the current development environment without modifying it.
func Inspect(ctx context.Context, config Config) (Report, error) {
	return inspect(ctx, hostSystem{}, config)
}

func inspect(ctx context.Context, sys system, config Config) (Report, error) {
	report := Report{Host: sys.GOOS() + "/" + sys.GOARCH()}

	sdkPath, sdkVersion, sdkErr := discoverSDK(sys, config.SDKPath)
	if sdkErr == nil {
		report.SDKPath = sdkPath
		report.SDKVersion = sdkVersion
	}

	toolNames := []string{"go", "tinygo", "cmake", "arm-none-eabi-gcc"}
	toolNames = append(toolNames, hostCompilerCandidates(sys.GOOS())...)
	seen := make(map[string]bool)
	for _, name := range toolNames {
		if seen[name] {
			continue
		}
		seen[name] = true
		if tool, ok := findTool(ctx, sys, name); ok {
			report.Tools = append(report.Tools, tool)
		}
	}
	if sdkErr == nil {
		for _, tool := range []Tool{
			{Name: "pdc", Path: sdkToolPath(sys.GOOS(), sdkPath, "pdc"), Version: sdkVersion},
			{Name: "pdutil", Path: sdkToolPath(sys.GOOS(), sdkPath, "pdutil")},
			{Name: "simulator", Path: simulatorPath(sys.GOOS(), sdkPath)},
		} {
			if isFile(sys, tool.Path) {
				report.Tools = append(report.Tools, tool)
			}
		}
	}

	sort.Slice(report.Tools, func(i, j int) bool { return report.Tools[i].Name < report.Tools[j].Name })
	report.Capabilities = assess(report, sdkErr, sys.GOOS())
	for _, capability := range report.Capabilities {
		if err := capability.validate(); err != nil {
			return Report{}, err
		}
	}
	return report, nil
}

func discoverSDK(sys system, explicit string) (string, string, error) {
	var candidates []sdkCandidate
	if explicit != "" {
		candidates = append(candidates, sdkCandidate{explicit, "--sdk"})
	} else if path, ok := sys.LookupEnv("PLAYDATE_SDK_PATH"); ok && strings.TrimSpace(path) != "" {
		candidates = append(candidates, sdkCandidate{path, "PLAYDATE_SDK_PATH"})
	} else {
		home, err := sys.HomeDir()
		if err == nil {
			candidates = append(candidates, conventionalSDKCandidates(sys.GOOS(), home)...)
		}
	}

	var problems []string
	for _, candidate := range candidates {
		root, err := filepath.Abs(filepath.Clean(candidate.path))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: invalid path", candidate.source))
			continue
		}
		versionBytes, err := sys.ReadFile(filepath.Join(root, "VERSION.txt"))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: VERSION.txt not found", candidate.source))
			continue
		}
		if !isFile(sys, sdkToolPath(sys.GOOS(), root, "pdc")) {
			problems = append(problems, fmt.Sprintf("%s: pdc not found", candidate.source))
			continue
		}
		version := strings.TrimSpace(string(versionBytes))
		if version == "" {
			problems = append(problems, fmt.Sprintf("%s: VERSION.txt is empty", candidate.source))
			continue
		}
		return root, version, nil
	}
	if len(problems) == 0 {
		return "", "", errors.New("playdate SDK not found; set PLAYDATE_SDK_PATH or pass --sdk")
	}
	return "", "", fmt.Errorf("playdate SDK not found (%s)", strings.Join(problems, "; "))
}

func conventionalSDKCandidates(goos, home string) []sdkCandidate {
	paths := []string{filepath.Join(home, "Developer", "PlaydateSDK")}
	if goos == "windows" {
		paths = append([]string{filepath.Join(home, "Documents", "PlaydateSDK")}, paths...)
	}
	result := make([]sdkCandidate, 0, len(paths))
	for _, path := range paths {
		result = append(result, sdkCandidate{path: path, source: "conventional path"})
	}
	return result
}

func isFile(sys system, path string) bool {
	info, err := sys.Stat(path)
	return err == nil && !info.IsDir()
}

func findTool(ctx context.Context, sys system, name string) (Tool, bool) {
	path, err := sys.LookPath(name)
	if err != nil {
		return Tool{}, false
	}
	tool := Tool{Name: name, Path: path}
	if host, ok := sys.(hostSystem); ok {
		_ = host
		command := exec.CommandContext(ctx, path, "version")
		if name == "cmake" || name == "arm-none-eabi-gcc" || name == "gcc" || name == "clang" || name == "cl" {
			command = exec.CommandContext(ctx, path, "--version")
		}
		if output, runErr := command.CombinedOutput(); runErr == nil {
			line, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")
			tool.Version = strings.TrimSpace(line)
		}
	}
	return tool, true
}

func hostCompilerCandidates(goos string) []string {
	policy, err := hostpolicy.For(goos)
	if err != nil {
		return nil
	}
	return policy.CompilerCandidates
}

func assess(report Report, sdkErr error, goos string) []Capability {
	profile := toolchainprofile.Accepted()
	capabilities := make([]Capability, 0, 5)
	if sdkErr != nil {
		capabilities = append(capabilities, Capability{"sdk", StatusMissing, sdkErr.Error()})
	} else if _, pdc := report.tool("pdc"); !pdc {
		capabilities = append(capabilities, Capability{"sdk", StatusIncompatible, "SDK is missing pdc"})
	} else if report.SDKVersion != profile.PlaydateSDK {
		capabilities = append(capabilities, Capability{"sdk", StatusUnverified, "Playdate SDK " + report.SDKVersion + "; verified profile uses " + profile.PlaydateSDK})
	} else {
		capabilities = append(capabilities, Capability{"sdk", StatusReady, "Playdate SDK " + report.SDKVersion})
	}

	if tool, ok := report.tool("go"); ok {
		summary := tool.Version
		if summary == "" {
			summary = tool.Path
		}
		status := StatusReady
		if tool.Version != "" && !matchesVerifiedTool("go", tool.Version) {
			status = StatusUnverified
			summary += "; verified profile uses go" + profile.Go
		}
		capabilities = append(capabilities, Capability{"develop", status, summary})
	} else {
		capabilities = append(capabilities, Capability{"develop", StatusMissing, "Go compiler not found on PATH"})
	}

	_, simulator := report.tool("simulator")
	compiler := firstTool(report, hostCompilerCandidates(goos))
	if sdkErr != nil || !simulator {
		capabilities = append(capabilities, Capability{"simulator", StatusMissing, "Playdate Simulator is unavailable"})
	} else if compiler == "" {
		capabilities = append(capabilities, Capability{"simulator", StatusMissing, "native C compiler not found on PATH"})
	} else {
		capabilities = append(capabilities, Capability{"simulator", StatusUnverified, compiler + " found; probe build has not run"})
	}

	_, tinygo := report.tool("tinygo")
	_, armgcc := report.tool("arm-none-eabi-gcc")
	switch {
	case !tinygo && !armgcc:
		capabilities = append(capabilities, Capability{"device-build", StatusMissing, "TinyGo and arm-none-eabi-gcc not found"})
	case !tinygo:
		capabilities = append(capabilities, Capability{"device-build", StatusMissing, "TinyGo not found"})
	case !armgcc:
		capabilities = append(capabilities, Capability{"device-build", StatusMissing, "arm-none-eabi-gcc not found"})
	default:
		summary := "toolchain found; probe build has not run"
		if detected := detectedToolchainSummary(report); detected != "" {
			summary = detected + "; verified profile: TinyGo " + profile.TinyGo + ", Arm GCC " + profile.ArmGCC + "; probe build has not run"
		}
		capabilities = append(capabilities, Capability{"device-build", StatusUnverified, summary})
	}

	if _, pdutil := report.tool("pdutil"); !pdutil {
		capabilities = append(capabilities, Capability{"device-deploy", StatusMissing, "pdutil is unavailable"})
	} else {
		capabilities = append(capabilities, Capability{"device-deploy", StatusUnverified, "pdutil found; device connectivity not probed"})
	}
	return capabilities
}

func firstTool(report Report, names []string) string {
	for _, name := range names {
		if _, ok := report.tool(name); ok {
			return name
		}
	}
	return ""
}
