package analyzer

import (
	"reflect"
	"testing"
)

func TestFilterSourceFindingsGeneratedAndChangedPolicies(t *testing.T) {
	snapshot := Snapshot{Packages: []Package{{Files: []SourceFile{
		{Path: "game.go", Roles: []SourceRole{RoleProduction}},
		{Path: "generated.go", Roles: []SourceRole{RoleProduction, RoleGenerated}},
	}}}}
	findings := []Finding{
		{RuleID: "device-goroutine", Position: "game.go:2:1", Fixes: []FindingFix{{Message: "fix"}}},
		{RuleID: "device-goroutine", Position: "generated.go:3:1", Fixes: []FindingFix{{Message: "unsafe generated edit"}}},
	}
	excluded, err := filterSourceFindings(snapshot, append([]Finding(nil), findings...), GeneratedExclude, nil)
	if err != nil || len(excluded) != 1 || excluded[0].Position != "game.go:2:1" {
		t.Fatalf("excluded = %+v, %v", excluded, err)
	}
	included, err := filterSourceFindings(snapshot, append([]Finding(nil), findings...), GeneratedInclude, nil)
	if err != nil || len(included) != 2 || len(included[1].Fixes) != 0 {
		t.Fatalf("included = %+v, %v", included, err)
	}
	changed, err := filterSourceFindings(snapshot, append([]Finding(nil), findings...), GeneratedInclude, map[string]bool{"generated.go": true})
	if err != nil || len(changed) != 1 || changed[0].Position != "generated.go:3:1" {
		t.Fatalf("changed = %+v, %v", changed, err)
	}
}

func TestNormalizeChangedFiles(t *testing.T) {
	files, err := normalizeChangedFiles([]string{"z.go", "dir/a.go", "z.go"})
	if err != nil || !reflect.DeepEqual(files, []string{"dir/a.go", "z.go"}) {
		t.Fatalf("files = %v, %v", files, err)
	}
	for _, invalid := range []string{"", ".", "../game.go", "/game.go", `dir\game.go`, "C:game.go", "dir/../game.go", "bad\x00name.go"} {
		if _, err := normalizeChangedFiles([]string{invalid}); err == nil {
			t.Fatalf("invalid path %q succeeded", invalid)
		}
	}
}

func TestBaselineStalenessRespectsChangedFiles(t *testing.T) {
	baseline := Baseline{Schema: BaselineSchema, Entries: []BaselineEntry{
		{Rule: "device-goroutine", Target: TargetDevice, Path: "changed.go", Line: 2, Column: 1, Message: "finding", Reason: "debt"},
		{Rule: "device-goroutine", Target: TargetDevice, Path: "unchanged.go", Line: 2, Column: 1, Message: "old", Reason: "debt"},
	}}
	findings := []Finding{{RuleID: "device-goroutine", Target: TargetDevice, Position: "changed.go:2:1", Message: "finding"}}
	if err := applyBaseline(baseline, findings, map[string]bool{"changed.go": true}); err != nil {
		t.Fatalf("scoped baseline = %v", err)
	}
	if findings[0].Suppression == nil || findings[0].Suppression.Kind != "baseline" {
		t.Fatalf("suppression = %+v", findings[0])
	}
}
