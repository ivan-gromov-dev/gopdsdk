package analyzer

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

// Providers exposes the shared, validated analysis prerequisites used by SDK
// rules. Rules require these analyzers instead of rebuilding syntax indexes,
// control flow, or SSA independently.
type Providers struct {
	Syntax      *analysis.Analyzer
	ControlFlow *analysis.Analyzer
	SSA         *analysis.Analyzer
}

// StandardProviders returns the canonical provider identities. Analyzer
// identity is significant to go/analysis ResultOf maps, so callers must retain
// and require these pointers rather than copying Analyzer values.
func StandardProviders() Providers {
	return Providers{Syntax: inspect.Analyzer, ControlFlow: ctrlflow.Analyzer, SSA: buildssa.Analyzer}
}
