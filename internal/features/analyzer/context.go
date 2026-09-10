package analyzer

import (
	"context"
	"sync"

	"golang.org/x/tools/go/analysis"
)

type deepAnalysisContextKey struct{}

func withDeepAnalysis(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, deepAnalysisContextKey{}, enabled)
}

func deepAnalysisEnabled(pass *analysis.Pass) bool {
	return deepAnalysisContext(PassContext(pass))
}

func deepAnalysisContext(ctx context.Context) bool {
	enabled, _ := ctx.Value(deepAnalysisContextKey{}).(bool)
	return enabled
}

var analysisPassContexts sync.Map

// PassContext returns the cancellation context for a pass run by Registry.
// Analyzer implementations should check it at bounded intervals during work
// that may outlive an editor request. A pass run by another driver receives a
// non-cancellable background context.
func PassContext(pass *analysis.Pass) context.Context {
	if pass != nil {
		if value, exists := analysisPassContexts.Load(pass); exists {
			return value.(context.Context)
		}
	}
	return context.Background()
}

func attachPassContext(pass *analysis.Pass, ctx context.Context) {
	analysisPassContexts.Store(pass, ctx)
}

func detachPassContext(pass *analysis.Pass) {
	analysisPassContexts.Delete(pass)
}
