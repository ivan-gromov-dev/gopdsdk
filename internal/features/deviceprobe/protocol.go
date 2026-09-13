package deviceprobe

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

const buildResultSchema = "gopdsdk-build/v1"

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

func writeStructuredBuildResult(out io.Writer, result Result) error {
	artifact := filepath.ToSlash(strings.ReplaceAll(result.Output, `\`, "/"))
	structured := structuredBuildResult{Schema: buildResultSchema, Target: "device", Package: result.Package, Artifact: artifact,
		Metrics: structuredMetrics{StaticRAMBytes: result.Metrics.StaticRAM, ELFBytes: result.Metrics.ELF, PDXBytes: result.Metrics.PDX}}
	return toolingprotocol.WriteResult(out, "build device", structured)
}

func writeStructuredBuildFailure(out io.Writer, ctx context.Context, err error) error {
	category, action := "build-failed", "inspect-device-toolchain"
	if ctx.Err() != nil {
		category, action = "cancelled", "retry"
	} else if strings.Contains(err.Error(), "output already exists") {
		category, action = "output-conflict", "select-output-or-force"
	}
	if writeErr := toolingprotocol.WriteFailure(out, "build device", category, &toolingprotocol.Remediation{Action: action}); writeErr != nil {
		return writeErr
	}
	return &toolingprotocol.SilentError{Err: err}
}
