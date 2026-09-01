package analyzer

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

const inlineSuppressionPrefix = "//gopdsdk:ignore "

type suppressionKey struct {
	Path string
	Line int
	Rule RuleID
}

// applyInlineSuppressions recognizes a directive on the finding line or on
// the immediately preceding line. The required grammar is:
//
//	//gopdsdk:ignore rule-id -- non-empty reason
func applyInlineSuppressions(snapshot Snapshot, catalog RuleCatalog, findings []Finding, activePaths map[string]bool, activeRules map[RuleID]bool) error {
	directives := make(map[suppressionKey]string)
	files := make(map[string]string)
	for _, pkg := range snapshot.Packages {
		for _, file := range pkg.Files {
			files[file.Path] = file.AbsolutePath
		}
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		content, err := snapshot.readFile(files[path])
		if err != nil {
			return fmt.Errorf("read inline suppressions from %q: %w", path, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse inline suppressions from %q: %w", path, err)
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				directive := comment.Text
				if !strings.Contains(directive, "gopdsdk:ignore") {
					continue
				}
				line := fset.Position(comment.Pos()).Line
				if !strings.HasPrefix(directive, inlineSuppressionPrefix) {
					return fmt.Errorf("%s:%d: malformed inline suppression", path, line)
				}
				body := strings.TrimPrefix(directive, inlineSuppressionPrefix)
				ruleText, reason, ok := strings.Cut(body, " -- ")
				ruleID := RuleID(strings.TrimSpace(ruleText))
				reason = strings.TrimSpace(reason)
				if !ok || !stableID.MatchString(string(ruleID)) || reason == "" {
					return fmt.Errorf("%s:%d: malformed inline suppression", path, line)
				}
				rule, exists := catalog.Rule(ruleID)
				if !exists {
					return fmt.Errorf("%s:%d: inline suppression names unknown rule %q", path, line, ruleID)
				}
				if !rule.Suppressible {
					return fmt.Errorf("%s:%d: rule %q is not suppressible", path, line, ruleID)
				}
				offset := fset.Position(comment.Pos()).Offset
				lineStart := bytes.LastIndexByte(content[:offset], '\n') + 1
				targetLine := line
				if strings.TrimSpace(string(content[lineStart:offset])) == "" {
					targetLine++
				}
				key := suppressionKey{path, targetLine, ruleID}
				if _, duplicate := directives[key]; duplicate {
					return fmt.Errorf("%s:%d: duplicate inline suppression for %q", path, line, ruleID)
				}
				directives[key] = reason
			}
		}
	}
	for index := range findings {
		path, point, err := parsePosition(findings[index].Position)
		if err != nil {
			return err
		}
		if reason, ok := directives[suppressionKey{path, point.Line, findings[index].RuleID}]; ok {
			delete(directives, suppressionKey{path, point.Line, findings[index].RuleID})
			findings[index].Suppression = &FindingSuppression{Kind: "inline", Reason: reason}
		}
	}
	var stale []string
	for key := range directives {
		if activePaths[key.Path] && activeRules[key.Rule] {
			stale = append(stale, fmt.Sprintf("%s:%d %s", key.Path, key.Line, key.Rule))
		}
	}
	if len(stale) != 0 {
		sort.Strings(stale)
		return fmt.Errorf("stale inline suppressions: %s", strings.Join(stale, "; "))
	}
	return nil
}
