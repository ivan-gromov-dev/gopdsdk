package analyzer

import (
	"fmt"
	"sort"
)

type AnalysisProfile string

const (
	ProfileDefault      AnalysisProfile = "default"
	ProfileExperimental AnalysisProfile = "experimental"
	ProfileDeep         AnalysisProfile = "deep"
)

// CheckPolicy controls rule selection, presentation severity, and process
// status independently.
type CheckPolicy struct {
	Profile           AnalysisProfile
	Rules             []RuleID
	Categories        []RuleFamily
	ExcludedRules     []RuleID
	SeverityOverrides map[string]Severity
	FailOn            string
}

func (policy CheckPolicy) Resolve(catalog RuleCatalog) (RuleSelection, map[RuleID]Severity, error) {
	if policy.Profile == "" {
		policy.Profile = ProfileDefault
	}
	if policy.Profile != ProfileDefault && policy.Profile != ProfileExperimental && policy.Profile != ProfileDeep {
		return RuleSelection{}, nil, fmt.Errorf("unknown analyzer profile %q", policy.Profile)
	}
	selection := RuleSelection{IDs: policy.Rules, Families: policy.Categories, ExcludedIDs: policy.ExcludedRules,
		IncludeExperimental: policy.Profile != ProfileDefault}
	rules, err := catalog.Select(selection)
	if err != nil {
		return RuleSelection{}, nil, err
	}
	if _, err := failRank(policy.FailOn); err != nil {
		return RuleSelection{}, nil, err
	}
	resolved := make(map[RuleID]Severity, len(rules))
	for _, rule := range rules {
		severity := rule.Default
		if override, ok := policy.SeverityOverrides[string(rule.Family)]; ok {
			severity = override
		}
		if override, ok := policy.SeverityOverrides[string(rule.ID)]; ok {
			severity = override
		}
		resolved[rule.ID] = severity
	}
	for selector, severity := range policy.SeverityOverrides {
		if !severity.valid() {
			return RuleSelection{}, nil, fmt.Errorf("invalid severity %q for %q", severity, selector)
		}
		if family := RuleFamily(selector); family.valid() {
			continue
		}
		if _, ok := catalog.Rule(RuleID(selector)); !ok {
			return RuleSelection{}, nil, fmt.Errorf("unknown severity selector %q", selector)
		}
	}
	return selection, resolved, nil
}

func applySeverity(report *Report, overrides map[RuleID]Severity) {
	for index := range report.Diagnostics {
		if severity, ok := overrides[report.Diagnostics[index].Rule]; ok {
			report.Diagnostics[index].Severity = severity
		}
	}
}

func reportFails(report Report, threshold string) bool {
	want, _ := failRank(threshold)
	for _, diagnostic := range report.Diagnostics {
		got, _ := failRank(string(diagnostic.Severity))
		if got >= want && want != 0 {
			return true
		}
	}
	return false
}

func failRank(value string) (int, error) {
	switch value {
	case "none":
		return 0, nil
	case "information":
		return 1, nil
	case "performance":
		return 2, nil
	case "warning":
		return 3, nil
	case "error":
		return 4, nil
	default:
		return 0, fmt.Errorf("invalid fail threshold %q", value)
	}
}

func splitRuleIDs(values []string) []RuleID {
	result := make([]RuleID, len(values))
	for i, value := range values {
		result[i] = RuleID(value)
	}
	return result
}
func splitFamilies(values []string) []RuleFamily {
	result := make([]RuleFamily, len(values))
	for i, value := range values {
		result[i] = RuleFamily(value)
	}
	return result
}
func sortedSeverityKeys(values map[string]Severity) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
