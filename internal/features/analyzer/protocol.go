package analyzer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ProtocolSchema is the first stable machine-readable check result schema.
// New optional fields may be added within v1; existing fields keep their
// meaning and required fields are only changed by a new schema version.
const ProtocolSchema = "gopdsdk-check/v1"

// Report is the versioned structured result consumed by CI and editors.
type Report struct {
	Schema          string       `json:"schema"`
	AnalyzerVersion string       `json:"analyzerVersion"`
	SDKVersion      string       `json:"sdkVersion"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

// Diagnostic is a stable, presentation-independent analyzer record.
type Diagnostic struct {
	Rule          RuleID       `json:"rule"`
	Category      RuleFamily   `json:"category"`
	Severity      Severity     `json:"severity"`
	Confidence    Confidence   `json:"confidence"`
	Target        Target       `json:"target"`
	Message       string       `json:"message"`
	Primary       SourceRange  `json:"primary"`
	Related       []Related    `json:"related"`
	Documentation string       `json:"documentation"`
	Suppression   *Suppression `json:"suppression,omitempty"`
	Edits         []EditGroup  `json:"edits"`
}

// SourceRange uses one-based lines and columns and a module-relative slash path.
type SourceRange struct {
	Path  string `json:"path"`
	Start Point  `json:"start"`
	End   Point  `json:"end"`
}

type Point struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Related struct {
	Message string      `json:"message"`
	Range   SourceRange `json:"range"`
}

type Suppression struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

type EditGroup struct {
	Message string `json:"message"`
	Edits   []Edit `json:"edits"`
}

type Edit struct {
	Range   SourceRange `json:"range"`
	NewText string      `json:"newText"`
}

// NewReport enriches kernel findings with immutable rule metadata.
func NewReport(catalog RuleCatalog, analyzerVersion, sdkVersion string, findings []Finding) (Report, error) {
	report := Report{Schema: ProtocolSchema, AnalyzerVersion: analyzerVersion, SDKVersion: sdkVersion, Diagnostics: make([]Diagnostic, 0, len(findings))}
	if analyzerVersion == "" || sdkVersion == "" {
		return Report{}, fmt.Errorf("analyzer and SDK versions are required")
	}
	for _, finding := range findings {
		rule, ok := catalog.Rule(finding.RuleID)
		if !ok {
			return Report{}, fmt.Errorf("finding references unknown rule %q", finding.RuleID)
		}
		primary, err := parseSourceRange(finding.Position, finding.End)
		if err != nil {
			return Report{}, fmt.Errorf("finding %q primary range: %w", finding.RuleID, err)
		}
		diagnostic := Diagnostic{Rule: finding.RuleID, Category: rule.Family, Severity: rule.Default, Confidence: rule.Confidence,
			Target: finding.Target, Message: finding.Message, Primary: primary, Documentation: documentationFor(rule, finding.URL), Related: []Related{}, Edits: []EditGroup{}}
		if finding.Suppression != nil {
			diagnostic.Suppression = &Suppression{Kind: finding.Suppression.Kind, Reason: finding.Suppression.Reason}
		}
		for _, item := range finding.Related {
			rangeValue, err := parseSourceRange(item.Position, item.End)
			if err != nil {
				return Report{}, fmt.Errorf("finding %q related range: %w", finding.RuleID, err)
			}
			diagnostic.Related = append(diagnostic.Related, Related{Message: item.Message, Range: rangeValue})
		}
		for _, fix := range finding.Fixes {
			group := EditGroup{Message: fix.Message, Edits: []Edit{}}
			for _, item := range fix.Edits {
				rangeValue, err := parseSourceRange(item.Position, item.End)
				if err != nil {
					return Report{}, fmt.Errorf("finding %q edit range: %w", finding.RuleID, err)
				}
				group.Edits = append(group.Edits, Edit{Range: rangeValue, NewText: item.NewText})
			}
			diagnostic.Edits = append(diagnostic.Edits, group)
		}
		report.Diagnostics = append(report.Diagnostics, diagnostic)
	}
	return report, nil
}

func documentationFor(rule Rule, diagnosticURL string) string {
	if diagnosticURL != "" {
		return diagnosticURL
	}
	if len(rule.ContractIDs) == 0 {
		return ""
	}
	return "docs/analyzer/rules/v1/" + string(rule.ID) + ".md"
}

func parseSourceRange(start, end string) (SourceRange, error) {
	if start == "" {
		return SourceRange{}, fmt.Errorf("start position is empty")
	}
	startPath, startPoint, err := parsePosition(start)
	if err != nil {
		return SourceRange{}, err
	}
	endPath, endPoint, err := parsePosition(end)
	if err != nil {
		return SourceRange{}, err
	}
	if endPath == "" {
		endPath, endPoint = startPath, startPoint
	}
	if startPath != endPath {
		return SourceRange{}, fmt.Errorf("range spans %q and %q", startPath, endPath)
	}
	return SourceRange{Path: startPath, Start: startPoint, End: endPoint}, nil
}

func parsePosition(value string) (string, Point, error) {
	if value == "" {
		return "", Point{}, nil
	}
	last := strings.LastIndexByte(value, ':')
	if last < 0 {
		return "", Point{}, fmt.Errorf("invalid position %q", value)
	}
	previous := strings.LastIndexByte(value[:last], ':')
	if previous < 0 {
		return "", Point{}, fmt.Errorf("invalid position %q", value)
	}
	line, lineErr := strconv.Atoi(value[previous+1 : last])
	column, columnErr := strconv.Atoi(value[last+1:])
	if lineErr != nil || columnErr != nil || line < 1 || column < 1 {
		return "", Point{}, fmt.Errorf("invalid position %q", value)
	}
	return value[:previous], Point{Line: line, Column: column}, nil
}

// JSON returns deterministic UTF-8 JSON terminated by one newline.
func (report Report) JSON() ([]byte, error) {
	if report.Schema != ProtocolSchema {
		return nil, fmt.Errorf("unsupported report schema %q", report.Schema)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode analyzer report: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeReport accepts unknown fields for forward compatibility while
// rejecting a schema version whose required semantics are unknown.
func DecodeReport(data []byte) (Report, error) {
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, fmt.Errorf("decode analyzer report: %w", err)
	}
	if report.Schema != ProtocolSchema {
		return Report{}, fmt.Errorf("unsupported report schema %q", report.Schema)
	}
	return report, nil
}
