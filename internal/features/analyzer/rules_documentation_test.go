package analyzer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStableRuleDocumentationIsVersionedAndCurrent(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate analyzer package")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	for _, rule := range ContractInventory().Rules {
		if rule.Experimental {
			continue
		}
		path := filepath.Join(root, "docs", "analyzer", "rules", "v1", string(rule.ID)+".md")
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("rule %q documentation: %v", rule.ID, err)
			continue
		}
		for _, required := range []string{"# `" + string(rule.ID) + "`", rule.Summary, string(rule.Default), rule.SafeFixPolicy, "//gopdsdk:ignore " + string(rule.ID)} {
			if !strings.Contains(strings.ToLower(string(contents)), strings.ToLower(required)) {
				t.Errorf("rule %q documentation is stale: missing %q", rule.ID, required)
			}
		}
		if got := documentationFor(rule, ""); got != "docs/analyzer/rules/v1/"+string(rule.ID)+".md" {
			t.Errorf("rule %q link = %q", rule.ID, got)
		}
	}
}
