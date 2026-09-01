package analyzer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckOutputGoldens(t *testing.T) {
	report := Report{Schema: ProtocolSchema, AnalyzerVersion: "v1", SDKVersion: "v1.0.0", Diagnostics: []Diagnostic{{
		Rule: "workspace-protocol-synthetic", Category: FamilyWorkspace, Severity: SeverityWarning, Confidence: ConfidenceProven, Target: TargetDevice,
		Message: "synthetic protocol finding", Primary: SourceRange{Path: "game/main.go", Start: Point{Line: 10, Column: 2}, End: Point{Line: 10, Column: 8}},
		Related:       []Related{{Message: "value originates here", Range: SourceRange{Path: "game/owner.go", Start: Point{Line: 4, Column: 1}, End: Point{Line: 4, Column: 6}}}},
		Documentation: "https://example.invalid/rule", Suppression: &Suppression{Kind: "inline", Reason: "fixture reason"},
		Edits: []EditGroup{{Message: "replace safely", Edits: []Edit{{Range: SourceRange{Path: "game/main.go", Start: Point{Line: 10, Column: 2}, End: Point{Line: 10, Column: 8}}, NewText: "fixed"}}}},
	}}}
	var text bytes.Buffer
	if err := writeTextReport(&text, report); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "check.txt", text.Bytes())
	structured, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "check.json", structured)
}

func assertGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	expected, err := os.ReadFile(filepath.Join("testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	expected = bytes.ReplaceAll(expected, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(actual, expected) {
		t.Fatalf("%s differs\n--- expected ---\n%s--- actual ---\n%s", name, expected, actual)
	}
}
