package analyzer

import "testing"

func TestCheckPolicySelectionSeverityAndThreshold(t *testing.T) {
	catalog := syntheticProtocolCatalog(t)
	policy := CheckPolicy{Profile: ProfileExperimental, Rules: []RuleID{"workspace-protocol-synthetic"},
		SeverityOverrides: map[string]Severity{"workspace": SeverityInformation, "workspace-protocol-synthetic": SeverityError}, FailOn: "warning"}
	selection, severities, err := policy.Resolve(catalog)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := catalog.Select(selection)
	if err != nil || len(selected) != 1 || selected[0].ID != "workspace-protocol-synthetic" {
		t.Fatalf("selection = %+v, %v", selected, err)
	}
	if severities["workspace-protocol-synthetic"] != SeverityError {
		t.Fatalf("rule override did not win: %+v", severities)
	}
	report := Report{Diagnostics: []Diagnostic{{Rule: "workspace-protocol-synthetic", Severity: SeverityWarning}}}
	applySeverity(&report, severities)
	if !reportFails(report, "warning") || reportFails(report, "none") {
		t.Fatalf("threshold result for %+v is invalid", report)
	}
}

func TestCheckPolicyRejectsUnknownSelectorsAndExcludesRules(t *testing.T) {
	catalog := syntheticProtocolCatalog(t)
	selection, _, err := (CheckPolicy{Profile: ProfileDefault, ExcludedRules: []RuleID{"workspace-protocol-synthetic"}, FailOn: "none"}).Resolve(catalog)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := catalog.Select(selection)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.ID == "workspace-protocol-synthetic" {
			t.Fatal("excluded rule was selected")
		}
	}
	for _, policy := range []CheckPolicy{
		{Profile: "unknown", FailOn: "warning"},
		{Profile: ProfileDefault, FailOn: "fatal"},
		{Profile: ProfileDefault, FailOn: "warning", SeverityOverrides: map[string]Severity{"missing-rule": SeverityError}},
	} {
		if _, _, err := policy.Resolve(catalog); err == nil {
			t.Fatalf("invalid policy %+v succeeded", policy)
		}
	}
}
