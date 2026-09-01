package analyzer

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestProtocolSyntheticFindingExercisesEveryField(t *testing.T) {
	catalog := syntheticProtocolCatalog(t)
	finding := Finding{
		RuleID: "workspace-protocol-synthetic", Analyzer: "protocolsynthetic", PackageID: "example.com/game", Target: TargetDevice,
		Position: "game/main.go:10:2", End: "game/main.go:10:8", Category: "synthetic", Message: "synthetic protocol finding", URL: "https://example.invalid/rule",
		Related: []RelatedFinding{{Position: "game/owner.go:4:1", End: "game/owner.go:4:6", Message: "value originates here"}},
		Fixes:   []FindingFix{{Message: "replace safely", Edits: []FindingEdit{{Position: "game/main.go:10:2", End: "game/main.go:10:8", NewText: "fixed"}}}},
	}
	report, err := NewReport(catalog, "v1.1.0", "v1.0.0", []Finding{finding})
	if err != nil {
		t.Fatal(err)
	}
	report.Diagnostics[0].Suppression = &Suppression{Kind: "inline", Reason: "fixture reason"}
	data, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) || bytes.Contains(data, []byte("\\\\")) {
		t.Fatalf("JSON is not normalized: %s", data)
	}
	decoded, err := DecodeReport(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, report) {
		t.Fatalf("round trip differs:\n%+v\n%+v", report, decoded)
	}
	diagnostic := decoded.Diagnostics[0]
	if diagnostic.Primary.Path != "game/main.go" || diagnostic.Primary.Start.Line != 10 || diagnostic.Related[0].Range.Path != "game/owner.go" || diagnostic.Edits[0].Edits[0].NewText != "fixed" || diagnostic.Suppression.Reason == "" {
		t.Fatalf("protocol fields missing: %+v", diagnostic)
	}
}

func TestDecodeReportAllowsUnknownFieldsAndRejectsUnknownSchema(t *testing.T) {
	data := []byte(`{"schema":"gopdsdk-check/v1","analyzerVersion":"v1.1.0","sdkVersion":"v1.0.0","diagnostics":[],"future":{"enabled":true}}`)
	if _, err := DecodeReport(data); err != nil {
		t.Fatalf("unknown field rejected: %v", err)
	}
	unknown := bytes.Replace(data, []byte(ProtocolSchema), []byte("gopdsdk-check/v2"), 1)
	if _, err := DecodeReport(unknown); err == nil || !strings.Contains(err.Error(), "unsupported report schema") {
		t.Fatalf("unknown schema error = %v", err)
	}
}

func TestProtocolRejectsMalformedAndCrossFileRanges(t *testing.T) {
	for _, finding := range []Finding{
		{RuleID: "workspace-protocol-synthetic", Target: TargetDevice, Position: "invalid", End: ""},
		{RuleID: "workspace-protocol-synthetic", Target: TargetDevice, Position: "a.go:1:1", End: "b.go:1:2"},
	} {
		if _, err := NewReport(syntheticProtocolCatalog(t), "v1.1.0", "v1.0.0", []Finding{finding}); err == nil {
			t.Fatalf("NewReport(%+v) succeeded", finding)
		}
	}
}

func syntheticProtocolCatalog(t *testing.T) RuleCatalog {
	t.Helper()
	inventory := ContractInventory()
	inventory.Rules = append(inventory.Rules, Rule{ID: "workspace-protocol-synthetic", Family: FamilyWorkspace, Summary: "exercise protocol fields", Default: SeverityWarning,
		Confidence: ConfidenceProven, Targets: []Target{TargetDevice}, ContractIDs: []string{inventory.Contracts[0].ID}, Suppressible: true, SafeFixPolicy: "test-only deterministic edits"})
	catalog, err := NewRuleCatalog(inventory)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
