package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

// ResultSchema identifies the stable structured doctor result.
const ResultSchema = "gopdsdk-doctor/v1"

// StructuredReport is the presentation-independent environment assessment.
type StructuredReport struct {
	Schema string        `json:"schema"`
	Host   string        `json:"host"`
	SDK    *SDKRecord    `json:"sdk,omitempty"`
	Tools  []ToolRecord  `json:"tools"`
	Checks []CheckRecord `json:"checks"`
}

type SDKRecord struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type ToolRecord struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
}

// CheckRecord separates prerequisite discovery from verified readiness.
type CheckRecord struct {
	ID              string       `json:"id"`
	Discovered      bool         `json:"discovered"`
	Status          Status       `json:"status"`
	EvidenceLevel   string       `json:"evidenceLevel"`
	FailureCategory string       `json:"failureCategory,omitempty"`
	Remediation     *Remediation `json:"remediation,omitempty"`
}

// Remediation describes a focused action without editor-specific presentation.
type Remediation struct {
	Action string `json:"action"`
	Value  string `json:"value,omitempty"`
}

func writeStructuredReport(out io.Writer, report Report) error {
	return toolingprotocol.WriteResult(out, "doctor", newStructuredReport(report))
}

// DecodeStructuredReport accepts additive fields and rejects unknown schema
// versions or multiple JSON values.
func DecodeStructuredReport(data []byte) (StructuredReport, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var report StructuredReport
	if err := decoder.Decode(&report); err != nil {
		return StructuredReport{}, fmt.Errorf("decode doctor result: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return StructuredReport{}, fmt.Errorf("decode doctor result: trailing JSON value")
	}
	if report.Schema != ResultSchema {
		return StructuredReport{}, fmt.Errorf("unsupported doctor result schema %q", report.Schema)
	}
	return report, nil
}

func newStructuredReport(report Report) StructuredReport {
	result := StructuredReport{Schema: ResultSchema, Host: report.Host, Tools: make([]ToolRecord, 0, len(report.Tools)), Checks: make([]CheckRecord, 0, len(report.Capabilities))}
	if report.SDKPath != "" {
		result.SDK = &SDKRecord{Path: normalizeStructuredPath(report.SDKPath), Version: report.SDKVersion}
	}
	for _, tool := range report.Tools {
		result.Tools = append(result.Tools, ToolRecord{Name: tool.Name, Path: normalizeStructuredPath(tool.Path), Version: tool.Version})
	}
	for _, capability := range report.Capabilities {
		result.Checks = append(result.Checks, structuredCheck(report, capability))
	}
	sort.Slice(result.Tools, func(i, j int) bool { return result.Tools[i].Name < result.Tools[j].Name })
	sort.Slice(result.Checks, func(i, j int) bool { return result.Checks[i].ID < result.Checks[j].ID })
	return result
}

func normalizeStructuredPath(path string) string {
	return filepath.ToSlash(strings.ReplaceAll(path, `\`, "/"))
}

func structuredCheck(report Report, capability Capability) CheckRecord {
	discovered := false
	switch capability.Name {
	case "sdk":
		discovered = report.SDKPath != ""
	case "develop":
		_, discovered = report.tool("go")
	case "simulator":
		_, discovered = report.tool("simulator")
	case "device-build":
		_, tinygo := report.tool("tinygo")
		_, gcc := report.tool("arm-none-eabi-gcc")
		discovered = tinygo && gcc
	case "device-deploy":
		_, discovered = report.tool("pdutil")
	}
	check := CheckRecord{ID: capability.Name, Discovered: discovered, Status: capability.Status, EvidenceLevel: evidenceLevel(capability)}
	switch capability.Status {
	case StatusMissing:
		check.FailureCategory = "not-found"
		check.Remediation = remediationFor(capability.Name, "install-or-configure")
	case StatusIncompatible:
		check.FailureCategory = "probe-failed"
		check.Remediation = remediationFor(capability.Name, "inspect-configuration")
	case StatusUnverified:
		if capability.Name == "sdk" || capability.Name == "develop" {
			check.FailureCategory = "unverified-version"
			check.Remediation = remediationFor(capability.Name, "install-verified-version")
		} else {
			check.FailureCategory = "not-probed"
			check.Remediation = remediationFor(capability.Name, "run-probe")
		}
	}
	return check
}

func evidenceLevel(capability Capability) string {
	if capability.Status != StatusReady {
		return "discovery"
	}
	switch capability.Name {
	case "simulator":
		return "sdk-integration"
	case "device-build":
		return "device-build"
	default:
		return "discovery"
	}
}

func remediationFor(check, fallback string) *Remediation {
	switch check {
	case "sdk":
		return &Remediation{Action: "configure-sdk-path", Value: "PLAYDATE_SDK_PATH"}
	case "simulator", "device-build":
		return &Remediation{Action: "run-doctor-probe", Value: check}
	case "device-deploy":
		return &Remediation{Action: "run-connection-probe"}
	default:
		return &Remediation{Action: fallback}
	}
}
