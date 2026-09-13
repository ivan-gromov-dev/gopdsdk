package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

func TestRunRulesIsDeterministicAndVersioned(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := RunRules(context.Background(), []string{"rules", "--format", "json"}, &first, &bytes.Buffer{}, options); err != nil {
		t.Fatal(err)
	}
	if err := RunRules(context.Background(), []string{"rules"}, &second, &bytes.Buffer{}, options); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("rule catalog is not deterministic")
	}
	envelope, err := toolingprotocol.DecodeEnvelope(first.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var inventory Inventory
	if err := json.Unmarshal(envelope.Result, &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.Schema != "gopdsdk-analyzer-contracts/v1" || len(inventory.Rules) == 0 {
		t.Fatalf("incomplete inventory: %+v", inventory)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RunRules(ctx, []string{"rules"}, &bytes.Buffer{}, &bytes.Buffer{}, options); err == nil {
		t.Fatal("cancelled rule catalog succeeded")
	}
}

func TestDecodeBaselineResultAllowsUnknownFieldsAndRejectsVersions(t *testing.T) {
	data := []byte(`{"schema":"gopdsdk-baseline-result/v1","operation":"validate","path":"baseline.json","entries":0,"staleEntries":[],"future":true}`)
	if _, err := DecodeBaselineResult(data); err != nil {
		t.Fatalf("unknown field rejected: %v", err)
	}
	unknown := bytes.Replace(data, []byte(BaselineResultSchema), []byte("gopdsdk-baseline-result/v2"), 1)
	if _, err := DecodeBaselineResult(unknown); err == nil || !strings.Contains(err.Error(), "unsupported baseline result schema") {
		t.Fatalf("unknown schema error = %v", err)
	}
}

func TestRunBaselineCreateUpdateAndValidateStaleEntries(t *testing.T) {
	root := t.TempDir()
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = root
	rule := firstSuppressibleRule(t, options.Catalog)
	reportPath := filepath.Join(root, "report.json")
	baselinePath := filepath.Join(root, "baseline.json")
	writeReportFixture(t, reportPath, rule, "first", 3)
	var output bytes.Buffer
	if err := RunBaseline(context.Background(), []string{"baseline", "create", "--input", "report.json", "--output", "baseline.json", "--reason", "initial debt"}, &output, &bytes.Buffer{}, options); err != nil {
		t.Fatal(err)
	}
	created, err := loadBaseline(root, baselinePath, options.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Entries) != 1 || created.Entries[0].Reason != "initial debt" {
		t.Fatalf("created baseline = %+v", created)
	}

	writeReportFixture(t, reportPath, rule, "second", 8)
	output.Reset()
	if err := RunBaseline(context.Background(), []string{"baseline", "validate", "--input", "report.json", "--output", "baseline.json"}, &output, &bytes.Buffer{}, options); err != nil {
		t.Fatal(err)
	}
	var validation BaselineResult
	decodeBaselineResult(t, output.Bytes(), &validation)
	if len(validation.StaleEntries) != 1 || validation.StaleEntries[0].Message != "first" {
		t.Fatalf("stale entries = %+v", validation.StaleEntries)
	}

	output.Reset()
	if err := RunBaseline(context.Background(), []string{"baseline", "update", "--input", "report.json", "--output", "baseline.json", "--reason", "new debt"}, &output, &bytes.Buffer{}, options); err != nil {
		t.Fatal(err)
	}
	updated, err := loadBaseline(root, baselinePath, options.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Entries) != 1 || updated.Entries[0].Message != "second" || updated.Entries[0].Reason != "new debt" {
		t.Fatalf("updated baseline = %+v", updated)
	}
	backups, err := filepath.Glob(filepath.Join(root, ".gopdsdk-baseline-backup-*"))
	if err != nil || len(backups) != 0 {
		t.Fatalf("backup remains: %v, %v", backups, err)
	}
}

func TestRunBaselineRejectsOverwriteAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = root
	rule := firstSuppressibleRule(t, options.Catalog)
	writeReportFixture(t, filepath.Join(root, "report.json"), rule, "finding", 1)
	args := []string{"baseline", "create", "--input", "report.json", "--output", "baseline.json"}
	if err := RunBaseline(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, options); err != nil {
		t.Fatal(err)
	}
	if err := RunBaseline(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, options); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("overwrite error = %v", err)
	}
	if err := RunBaseline(context.Background(), []string{"baseline", "create", "--input", "report.json", "--output", "../outside.json"}, &bytes.Buffer{}, &bytes.Buffer{}, options); err == nil {
		t.Fatal("escaping output accepted")
	}
}

func firstSuppressibleRule(t *testing.T, catalog RuleCatalog) Rule {
	t.Helper()
	rules, err := catalog.Select(RuleSelection{})
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.Suppressible {
			return rule
		}
	}
	t.Fatal("no suppressible rule")
	return Rule{}
}

func writeReportFixture(t *testing.T, path string, rule Rule, message string, line int) {
	t.Helper()
	report := Report{Schema: ProtocolSchema, AnalyzerVersion: "v1", SDKVersion: "v1.1.0", Diagnostics: []Diagnostic{{Rule: rule.ID, Category: rule.Family, Severity: rule.Default, Confidence: rule.Confidence, Target: rule.Targets[0], Message: message, Primary: SourceRange{Path: "game.go", Start: Point{Line: line, Column: 1}, End: Point{Line: line, Column: 2}}, Related: []Related{}, Edits: []EditGroup{}}}}
	data, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func decodeBaselineResult(t *testing.T, data []byte, result *BaselineResult) {
	t.Helper()
	envelope, err := toolingprotocol.DecodeEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != BaselineResultSchema {
		t.Fatalf("schema = %q", result.Schema)
	}
}
