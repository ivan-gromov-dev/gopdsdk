package deviceprobe

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/tooldiagnostic"
)

const buildResultSchema = "gopdsdk-build/v1"
const runResultSchema = "gopdsdk-run/v1"

type structuredBuildResult struct {
	Schema   string            `json:"schema"`
	Target   string            `json:"target"`
	Package  string            `json:"package"`
	Artifact string            `json:"artifact"`
	Metrics  structuredMetrics `json:"metrics"`
}

type structuredMetrics struct {
	StaticRAMBytes uint64 `json:"staticRAMBytes"`
	ELFBytes       int64  `json:"elfBytes"`
	PDXBytes       int64  `json:"pdxBytes"`
}

type structuredRunResult struct {
	Schema     string            `json:"schema"`
	Target     string            `json:"target"`
	Package    string            `json:"package"`
	Deployment string            `json:"deployment"`
	Execution  string            `json:"execution"`
	Metrics    structuredMetrics `json:"metrics"`
}

func writeStructuredBuildResult(out io.Writer, result Result) error {
	artifact := filepath.ToSlash(strings.ReplaceAll(result.Output, `\`, "/"))
	structured := structuredBuildResult{Schema: buildResultSchema, Target: "device", Package: result.Package, Artifact: artifact,
		Metrics: structuredMetrics{StaticRAMBytes: result.Metrics.StaticRAM, ELFBytes: result.Metrics.ELF, PDXBytes: result.Metrics.PDX}}
	return toolingprotocol.WriteResult(out, "build device", structured)
}

func writeStructuredRunResult(out io.Writer, result Result) error {
	structured := structuredRunResult{Schema: runResultSchema, Target: "device", Package: result.Package, Deployment: result.Deploy, Execution: result.Run,
		Metrics: structuredMetrics{StaticRAMBytes: result.Metrics.StaticRAM, ELFBytes: result.Metrics.ELF, PDXBytes: result.Metrics.PDX}}
	return toolingprotocol.WriteResult(out, "run device", structured)
}

func writeStructuredRunFailure(out io.Writer, ctx context.Context, err error, stage string) error {
	category, action := "build-failed", "inspect-device-toolchain"
	switch stage {
	case "deployment":
		category, action = "deployment-failed", "check-device-connection"
	case "launch":
		category, action = "launch-failed", "check-device-state"
	}
	if ctx.Err() != nil {
		category, action = "cancelled", "retry"
	}
	locations := protocolLocations(err)
	if writeErr := toolingprotocol.WriteFailureWithLocations(out, "run device", category, &toolingprotocol.Remediation{Action: action}, locations); writeErr != nil {
		return writeErr
	}
	return &toolingprotocol.SilentError{Err: err}
}

func writeStructuredBuildFailure(out io.Writer, ctx context.Context, err error) error {
	category, action := "build-failed", "inspect-device-toolchain"
	if ctx.Err() != nil {
		category, action = "cancelled", "retry"
	} else if strings.Contains(err.Error(), "output already exists") {
		category, action = "output-conflict", "select-output-or-force"
	} else if command := tooldiagnostic.Action(err); command != "" {
		switch {
		case strings.HasPrefix(command, "compile"), strings.HasPrefix(command, "resolve"):
			category = "compilation-failed"
		case strings.HasPrefix(command, "link"):
			category = "link-failed"
		case strings.HasPrefix(command, "package"):
			category = "packaging-failed"
		}
	}
	if writeErr := toolingprotocol.WriteFailureWithLocations(out, "build device", category, &toolingprotocol.Remediation{Action: action}, protocolLocations(err)); writeErr != nil {
		return writeErr
	}
	return &toolingprotocol.SilentError{Err: err}
}

func protocolLocations(err error) []toolingprotocol.SourceLocation {
	var result []toolingprotocol.SourceLocation
	for _, location := range tooldiagnostic.Locations(err) {
		result = append(result, toolingprotocol.SourceLocation{Path: location.Path, Line: location.Line, Column: location.Column})
	}
	return result
}
