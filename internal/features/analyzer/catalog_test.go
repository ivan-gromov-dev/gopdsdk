package analyzer

import (
	"reflect"
	"strings"
	"testing"
)

func TestRuleCatalogDefaultSelectionIsStableAndDeterministic(t *testing.T) {
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	first, err := catalog.Select(RuleSelection{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := catalog.Select(RuleSelection{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default selection is not deterministic")
	}
	for index, rule := range first {
		if rule.Experimental {
			t.Fatalf("default selection contains experimental rule %q", rule.ID)
		}
		if index > 0 && first[index-1].ID >= rule.ID {
			t.Fatalf("rules are not ordered: %q before %q", first[index-1].ID, rule.ID)
		}
	}
}

func TestRuleCatalogSelectsFamilyAndExplicitExperimentalRule(t *testing.T) {
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	device, err := catalog.Select(RuleSelection{Families: []RuleFamily{FamilyDevice}})
	if err != nil {
		t.Fatal(err)
	}
	if len(device) == 0 {
		t.Fatal("device selection is empty")
	}
	seenDevice := make(map[RuleID]bool)
	for _, rule := range device {
		if rule.Family != FamilyDevice {
			t.Fatalf("device selection contains %+v", rule)
		}
		seenDevice[rule.ID] = true
	}
	if !seenDevice["device-goroutine"] || !seenDevice["device-encoding-json"] {
		t.Fatalf("device selection is missing baseline rules: %+v", device)
	}
	performance, err := catalog.Select(RuleSelection{IDs: []RuleID{"performance-frame-allocation"}, IncludeExperimental: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(performance) != 1 || performance[0].ID != "performance-frame-allocation" {
		t.Fatalf("experimental selection = %+v", performance)
	}
}

func TestRuleCatalogRejectsUnknownAndImplicitExperimentalRules(t *testing.T) {
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	for name, selection := range map[string]RuleSelection{
		"unknown rule":                {IDs: []RuleID{"device-does-not-exist"}},
		"unknown family":              {Families: []RuleFamily{"missing"}},
		"experimental without opt-in": {IDs: []RuleID{"performance-frame-allocation"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := catalog.Select(selection); err == nil {
				t.Fatal("Select() succeeded")
			}
		})
	}
}

func TestRuleCatalogReturnsIndependentMetadata(t *testing.T) {
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := catalog.Rule("device-goroutine")
	if !ok {
		t.Fatal("device-goroutine is missing")
	}
	rule.Targets[0] = TargetSimulator
	rule.ContractIDs[0] = "changed"
	again, _ := catalog.Rule("device-goroutine")
	if again.Targets[0] != TargetDevice || strings.Contains(strings.Join(again.ContractIDs, ","), "changed") {
		t.Fatal("catalog exposed mutable rule metadata")
	}
}

func TestInventoryRejectsRuleOutsideDeclaredFamily(t *testing.T) {
	inventory := ContractInventory()
	inventory.Rules[0].Family = FamilyWorkspace
	if err := inventory.Validate(); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("Validate() = %v, want family mismatch", err)
	}
}
