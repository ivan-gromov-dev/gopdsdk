package analyzer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
)

const BaselineSchema = "gopdsdk-check-baseline/v1"

type Baseline struct {
	Schema  string          `json:"schema"`
	Entries []BaselineEntry `json:"entries"`
}

type BaselineEntry struct {
	Rule    RuleID `json:"rule"`
	Target  Target `json:"target"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

func loadBaseline(moduleRoot, name string, catalog RuleCatalog) (Baseline, error) {
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(moduleRoot, path)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Baseline{}, fmt.Errorf("read check baseline %q: %w", filepath.ToSlash(path), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var baseline Baseline
	if err := decoder.Decode(&baseline); err != nil {
		return Baseline{}, fmt.Errorf("decode check baseline %q: %w", filepath.ToSlash(path), err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return Baseline{}, fmt.Errorf("decode check baseline %q: %w", filepath.ToSlash(path), err)
	}
	if err := baseline.Validate(catalog); err != nil {
		return Baseline{}, fmt.Errorf("validate check baseline %q: %w", filepath.ToSlash(path), err)
	}
	return baseline, nil
}

func (baseline Baseline) Validate(catalog RuleCatalog) error {
	if baseline.Schema != BaselineSchema {
		return fmt.Errorf("unsupported schema %q", baseline.Schema)
	}
	seen := make(map[baselineKey]bool, len(baseline.Entries))
	for index, entry := range baseline.Entries {
		rule, ok := catalog.Rule(entry.Rule)
		if !ok {
			return fmt.Errorf("entry %d names unknown rule %q", index, entry.Rule)
		}
		if !rule.Suppressible {
			return fmt.Errorf("entry %d names non-suppressible rule %q", index, entry.Rule)
		}
		if entry.Target != TargetShared && entry.Target != TargetSimulator && entry.Target != TargetDevice {
			return fmt.Errorf("entry %d has invalid target %q", index, entry.Target)
		}
		if entry.Path == "" || entry.Path != filepath.ToSlash(entry.Path) || entry.Path != pathpkg.Clean(entry.Path) || filepath.IsAbs(entry.Path) || strings.HasPrefix(entry.Path, "/") || strings.Contains(entry.Path, ":") || entry.Path == ".." || strings.HasPrefix(entry.Path, "../") {
			return fmt.Errorf("entry %d has invalid module-relative path %q", index, entry.Path)
		}
		if entry.Line < 1 || entry.Column < 1 || entry.Message == "" || strings.TrimSpace(entry.Reason) == "" {
			return fmt.Errorf("entry %d is incomplete", index)
		}
		key := entry.key()
		if seen[key] {
			return fmt.Errorf("entry %d duplicates an earlier baseline identity", index)
		}
		seen[key] = true
	}
	return nil
}

type baselineKey struct {
	Rule         RuleID
	Target       Target
	Path         string
	Line, Column int
	Message      string
}

func (entry BaselineEntry) key() baselineKey {
	return baselineKey{entry.Rule, entry.Target, entry.Path, entry.Line, entry.Column, entry.Message}
}

func applyBaseline(baseline Baseline, findings []Finding) error {
	entries := make(map[baselineKey]BaselineEntry, len(baseline.Entries))
	for _, entry := range baseline.Entries {
		entries[entry.key()] = entry
	}
	used := make(map[baselineKey]bool, len(entries))
	for index := range findings {
		path, point, err := parsePosition(findings[index].Position)
		if err != nil {
			return err
		}
		key := baselineKey{findings[index].RuleID, findings[index].Target, path, point.Line, point.Column, findings[index].Message}
		if entry, ok := entries[key]; ok {
			used[key] = true
			if findings[index].Suppression == nil {
				findings[index].Suppression = &FindingSuppression{Kind: "baseline", Reason: entry.Reason}
			}
		}
	}
	var stale []string
	for _, entry := range baseline.Entries {
		if !used[entry.key()] {
			stale = append(stale, fmt.Sprintf("%s:%d:%d %s [%s]", entry.Path, entry.Line, entry.Column, entry.Rule, entry.Target))
		}
	}
	if len(stale) != 0 {
		sort.Strings(stale)
		return errors.New("stale baseline entries: " + strings.Join(stale, "; "))
	}
	return nil
}
