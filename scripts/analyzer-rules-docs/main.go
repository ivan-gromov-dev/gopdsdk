// Command analyzer-rules-docs generates the versioned analyzer rule reference.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/analyzer"
)

func main() {
	inventory := analyzer.ContractInventory()
	if err := inventory.Validate(); err != nil {
		panic(err)
	}
	directory := filepath.Join("docs", "analyzer", "rules", "v1")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		panic(err)
	}
	rules := append([]analyzer.Rule(nil), inventory.Rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	var index strings.Builder
	index.WriteString("# Analyzer rules v1\n\nThis reference is generated from the versioned analyzer contract inventory.\nStable rules are enabled by the default profile; experimental rules require\n`--profile experimental` or `--profile deep`.\n\n")
	for _, rule := range rules {
		if rule.Experimental {
			continue
		}
		name := string(rule.ID) + ".md"
		fmt.Fprintf(&index, "- [`%s`](%s) — %s.\n", rule.ID, name, rule.Summary)
		var page strings.Builder
		fmt.Fprintf(&page, "# `%s`\n\n%s.\n\n", rule.ID, sentence(rule.Summary))
		fmt.Fprintf(&page, "- Rule version: `v1`\n- Category: `%s`\n- Default severity: `%s`\n- Confidence: `%s`\n- Targets: `%s`\n- Suppressible: `%t`\n\n", rule.Family, rule.Default, rule.Confidence, joinTargets(rule.Targets), rule.Suppressible)
		page.WriteString("## Contract\n\n")
		fmt.Fprintf(&page, "This diagnostic enforces: `%s`. See the matching public contract in [API.md](../../../../API.md).\n\n", strings.Join(rule.ContractIDs, "`, `"))
		page.WriteString("## Safe fix policy\n\n")
		fmt.Fprintf(&page, "%s. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.\n\n", sentence(rule.SafeFixPolicy))
		page.WriteString("## Suppression\n\n")
		fmt.Fprintf(&page, "Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore %s -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.\n", rule.ID)
		if err := os.WriteFile(filepath.Join(directory, name), []byte(page.String()), 0o644); err != nil {
			panic(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte(index.String()), 0o644); err != nil {
		panic(err)
	}
}

func sentence(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + strings.TrimSuffix(value[1:], ".")
}

func joinTargets(targets []analyzer.Target) string {
	values := make([]string, len(targets))
	for index, target := range targets {
		values[index] = string(target)
	}
	return strings.Join(values, "`, `")
}
