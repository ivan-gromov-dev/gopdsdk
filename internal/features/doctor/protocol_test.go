package doctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

func TestStructuredReportSeparatesDiscoveryAndReadiness(t *testing.T) {
	report := Report{Host: "windows/amd64", SDKPath: `C:\Users\dev\PlaydateSDK`, SDKVersion: "3.1.1", Tools: []Tool{
		{Name: "simulator", Path: `C:\SDK\bin\PlaydateSimulator.exe`},
		{Name: "pdutil", Path: `C:\SDK\bin\pdutil.exe`},
	}, Capabilities: []Capability{
		{Name: "simulator", Status: StatusUnverified, Summary: `compiler found at C:\secret\tool`},
		{Name: "device-deploy", Status: StatusUnverified, Summary: "pdutil found"},
		{Name: "device-build", Status: StatusMissing, Summary: "TinyGo not found"},
	}}
	structured := newStructuredReport(report)
	if structured.SDK == nil || strings.Contains(structured.SDK.Path, `\`) {
		t.Fatalf("SDK path is not normalized: %+v", structured.SDK)
	}
	if structured.Checks[0].ID != "device-build" || structured.Checks[0].Discovered || structured.Checks[0].FailureCategory != "not-found" {
		t.Fatalf("device build check = %+v", structured.Checks[0])
	}
	if structured.Checks[2].ID != "simulator" || !structured.Checks[2].Discovered || structured.Checks[2].Status != StatusUnverified || structured.Checks[2].EvidenceLevel != "discovery" {
		t.Fatalf("simulator check = %+v", structured.Checks[2])
	}
	encoded, err := json.Marshal(structured)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("secret")) {
		t.Fatalf("structured report leaked raw diagnostic: %s", encoded)
	}
}

func TestRunJSONUsesToolingEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run(t.Context(), []string{"doctor", "--format", "json", "--sdk", t.TempDir()}, &stdout, &stderr, Options{}); err != nil {
		t.Fatal(err)
	}
	envelope, err := toolingprotocol.DecodeEnvelope(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Command != "doctor" || !envelope.OK || stderr.Len() != 0 {
		t.Fatalf("envelope = %+v, stderr = %q", envelope, stderr.String())
	}
	var report StructuredReport
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatal(err)
	}
	if report.Schema != ResultSchema || report.Host == "" || report.Tools == nil || report.Checks == nil {
		t.Fatalf("incomplete doctor result: %+v", report)
	}
}

func TestRunRejectsUnknownFormat(t *testing.T) {
	err := Run(t.Context(), []string{"doctor", "--format", "xml"}, &bytes.Buffer{}, &bytes.Buffer{}, Options{})
	if err == nil || !strings.Contains(err.Error(), "unsupported doctor format") {
		t.Fatalf("Run error = %v", err)
	}
}

func TestDecodeStructuredReportAllowsUnknownFieldsAndRejectsVersions(t *testing.T) {
	data := []byte(`{"schema":"gopdsdk-doctor/v1","host":"linux/amd64","tools":[],"checks":[],"future":true}`)
	if _, err := DecodeStructuredReport(data); err != nil {
		t.Fatalf("unknown field rejected: %v", err)
	}
	unknown := bytes.Replace(data, []byte(ResultSchema), []byte("gopdsdk-doctor/v2"), 1)
	if _, err := DecodeStructuredReport(unknown); err == nil || !strings.Contains(err.Error(), "unsupported doctor result schema") {
		t.Fatalf("unknown schema error = %v", err)
	}
}
