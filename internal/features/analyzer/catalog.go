package analyzer

import (
	"fmt"
	"sort"
)

// RuleSelection chooses rule metadata for an analyzer run. An empty ID and
// family selection means every stable rule.
type RuleSelection struct {
	IDs                 []RuleID
	Families            []RuleFamily
	ExcludedIDs         []RuleID
	IncludeExperimental bool
}

// RuleCatalog is an immutable, deterministic view of the inventory's rule
// registry. Step 1 attaches go/analysis implementations to these identities.
type RuleCatalog struct {
	rules map[RuleID]Rule
}

// NewRuleCatalog validates the complete inventory before exposing its rules.
func NewRuleCatalog(inventory Inventory) (RuleCatalog, error) {
	if err := inventory.Validate(); err != nil {
		return RuleCatalog{}, err
	}
	rules := make(map[RuleID]Rule, len(inventory.Rules))
	for _, rule := range inventory.Rules {
		rules[rule.ID] = cloneRule(rule)
	}
	return RuleCatalog{rules: rules}, nil
}

// Rule returns independent metadata for id.
func (catalog RuleCatalog) Rule(id RuleID) (Rule, bool) {
	rule, ok := catalog.rules[id]
	return cloneRule(rule), ok
}

// Select validates an explicit selection and returns matching rules ordered by
// stable identifier. Experimental rules require an explicit opt-in even when
// named directly.
func (catalog RuleCatalog) Select(selection RuleSelection) ([]Rule, error) {
	selectedIDs := make(map[RuleID]bool, len(selection.IDs))
	for _, id := range selection.IDs {
		if _, exists := catalog.rules[id]; !exists {
			return nil, fmt.Errorf("unknown analyzer rule %q", id)
		}
		selectedIDs[id] = true
	}
	selectedFamilies := make(map[RuleFamily]bool, len(selection.Families))
	for _, family := range selection.Families {
		if !family.valid() {
			return nil, fmt.Errorf("unknown analyzer rule family %q", family)
		}
		selectedFamilies[family] = true
	}
	excluded := make(map[RuleID]bool, len(selection.ExcludedIDs))
	for _, id := range selection.ExcludedIDs {
		if _, exists := catalog.rules[id]; !exists {
			return nil, fmt.Errorf("unknown excluded analyzer rule %q", id)
		}
		excluded[id] = true
	}
	selectAll := len(selectedIDs) == 0 && len(selectedFamilies) == 0
	rules := make([]Rule, 0, len(catalog.rules))
	for id, rule := range catalog.rules {
		if excluded[id] {
			continue
		}
		if !selectAll && !selectedIDs[id] && !selectedFamilies[rule.Family] {
			continue
		}
		if rule.Experimental && !selection.IncludeExperimental {
			if selectedIDs[id] {
				return nil, fmt.Errorf("analyzer rule %q is experimental and requires explicit opt-in", id)
			}
			continue
		}
		rules = append(rules, cloneRule(rule))
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

func cloneRule(rule Rule) Rule {
	rule.Targets = append([]Target(nil), rule.Targets...)
	rule.ContractIDs = append([]string(nil), rule.ContractIDs...)
	return rule
}
