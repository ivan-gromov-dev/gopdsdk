package analyzer

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

func TestDeviceRulePackReportsDocumentedProfileViolations(t *testing.T) {
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{"./noncompliant"}, Target: TargetDevice})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotHasLoadErrors(snapshot) {
		t.Fatalf("fixture load errors: %+v", snapshot.Packages)
	}
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(catalog, deviceRuleRegistrations()...)
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyDevice}})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[RuleID]bool)
	for _, finding := range result.Findings {
		seen[finding.RuleID] = true
	}
	var got []RuleID
	for rule := range seen {
		got = append(got, rule)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	want := []RuleID{"device-channel", "device-encoding-json", "device-finalizer", "device-fmt", "device-goroutine", "device-recover", "device-reflect-symbol", "device-runtime-control", "device-select", "device-time-runtime"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reported rules = %v, want %v", got, want)
	}
}

func TestDeviceRulePackAcceptsDocumentedSubsetAndOtherTargets(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		pattern string
		target  Target
	}{
		{name: "documented device subset", pattern: "./compliant", target: TargetDevice},
		{name: "noncompliant code is valid for simulator", pattern: "./noncompliant", target: TargetSimulator},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{test.pattern}, Target: test.target})
			if err != nil {
				t.Fatal(err)
			}
			result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyDevice}})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Findings) != 0 {
				t.Fatalf("findings = %+v, want none", result.Findings)
			}
		})
	}
}

func TestDefaultCheckOptionsRegistersEveryInitialDeviceRule(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := options.Catalog.Select(RuleSelection{Families: []RuleFamily{FamilyDevice}})
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.Family == FamilyDevice && options.Registry.registrations[rule.ID] == nil {
			t.Errorf("device rule %q is not registered", rule.ID)
		}
	}
}
