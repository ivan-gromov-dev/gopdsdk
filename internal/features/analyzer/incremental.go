package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// AnalysisSchedule identifies the amount of work appropriate at an interactive
// boundary. Fast edits and saves never enable the deep profile implicitly.
type AnalysisSchedule string

const (
	ScheduleFastEdit AnalysisSchedule = "fast-edit"
	ScheduleSave     AnalysisSchedule = "save"
	ScheduleDeep     AnalysisSchedule = "deep"
)

// Progress is a monotonic update for one incremental request.
type Progress struct {
	Stage   string
	Current int
	Total   int
}

// IncrementalRequest describes one immutable editor snapshot. Version must
// increase within a workspace; an older request is rejected as stale.
type IncrementalRequest struct {
	LoadConfig
	Version      uint64
	Schedule     AnalysisSchedule
	Selection    RuleSelection
	Generated    GeneratedPolicy
	ChangedFiles []string
	GopdsdkFloor string
	PlaydateSDK  string
	Progress     func(Progress)
}

// IncrementalMetrics records observable engine costs without retaining source
// text or environment values.
type IncrementalMetrics struct {
	ColdTime          time.Duration
	IncrementalTime   time.Duration
	CancellationDelay time.Duration
	PeakMemoryBytes   uint64
	CacheHits         uint64
	CacheMisses       uint64
	InvalidatedItems  int
}

// IncrementalResult is a batch-equivalent result for one accepted snapshot.
// Package load errors are partial workspace failures rather than engine errors.
type IncrementalResult struct {
	Version    uint64
	Findings   []Finding
	LoadErrors []LoadError
	Metrics    IncrementalMetrics
}

// ErrStaleAnalysis identifies a request superseded by a newer workspace
// version. Callers must not publish its diagnostics.
var ErrStaleAnalysis = errors.New("stale analyzer request")

// IncrementalOptions bounds retained snapshots. The default is four entries;
// eviction is deterministic least-recently-used eviction.
type IncrementalOptions struct {
	MaxSnapshots int
}

type incrementalEntry struct {
	key      string
	snapshot Snapshot
	findings []Finding
	loads    []LoadError
	used     uint64
}

// IncrementalEngine owns bounded, concurrency-safe workspace analysis state.
// Cached snapshots cover parsing and typing; cached results cover analyzer
// prerequisites (including SSA), facts, summaries, contracts, manifests, and
// assets. Entries are invalidated as a unit so no value can cross snapshots.
type IncrementalEngine struct {
	catalog  RuleCatalog
	registry Registry
	limit    int

	mu      sync.Mutex
	clock   uint64
	latest  map[string]uint64
	entries map[string]*incrementalEntry
}

func NewIncrementalEngine(catalog RuleCatalog, registry Registry, options IncrementalOptions) *IncrementalEngine {
	limit := options.MaxSnapshots
	if limit <= 0 {
		limit = 4
	}
	return &IncrementalEngine{catalog: catalog, registry: registry, limit: limit, latest: make(map[string]uint64), entries: make(map[string]*incrementalEntry)}
}

// Analyze loads and evaluates one request. Concurrent calls are safe. A result
// is published only if its version is still current when analysis completes.
func (engine *IncrementalEngine) Analyze(ctx context.Context, request IncrementalRequest) (IncrementalResult, error) {
	started := time.Now()
	canceledAt := make(chan time.Time, 1)
	watchDone := make(chan struct{})
	if ctx.Done() != nil {
		go func() {
			select {
			case <-ctx.Done():
				canceledAt <- time.Now()
			case <-watchDone:
			}
		}()
	}
	defer close(watchDone)
	canceledResult := func(err error) (IncrementalResult, error) {
		if contextErr := ctx.Err(); contextErr != nil {
			observed := time.Now()
			select {
			case observed = <-canceledAt:
			default:
			}
			return IncrementalResult{Version: request.Version, Metrics: IncrementalMetrics{CancellationDelay: time.Since(observed)}}, contextErr
		}
		return IncrementalResult{}, err
	}
	if request.Schedule != ScheduleFastEdit && request.Schedule != ScheduleSave && request.Schedule != ScheduleDeep {
		return IncrementalResult{}, fmt.Errorf("invalid analysis schedule %q", request.Schedule)
	}
	root, err := validateLoadConfig(request.LoadConfig)
	if err != nil {
		return IncrementalResult{}, err
	}
	workspace := strings.ToLower(filepath.Clean(root))
	engine.mu.Lock()
	if request.Version < engine.latest[workspace] {
		engine.mu.Unlock()
		return IncrementalResult{}, ErrStaleAnalysis
	}
	if request.Version > engine.latest[workspace] {
		engine.latest[workspace] = request.Version
	}
	engine.mu.Unlock()
	progress(request.Progress, "fingerprint", 0, 3)
	key, err := incrementalKey(ctx, root, request)
	if err != nil {
		return canceledResult(err)
	}
	if cached := engine.cached(key); cached != nil {
		if err := engine.current(workspace, request.Version); err != nil {
			return IncrementalResult{}, err
		}
		return IncrementalResult{Version: request.Version, Findings: cloneFindings(cached.findings), LoadErrors: append([]LoadError(nil), cached.loads...), Metrics: IncrementalMetrics{IncrementalTime: time.Since(started), CacheHits: 1}}, nil
	}
	progress(request.Progress, "load", 1, 3)
	snapshot, err := LoadPackages(ctx, request.LoadConfig)
	if err != nil {
		return canceledResult(err)
	}
	loads := incrementalLoadErrors(snapshot)
	var findings []Finding
	if len(loads) == 0 {
		progress(request.Progress, "analyze", 2, 3)
		analysisContext := withDeepAnalysis(ctx, request.Schedule == ScheduleDeep)
		run, runErr := engine.registry.Run(analysisContext, snapshot, request.Selection)
		if runErr != nil {
			return canceledResult(runErr)
		}
		findings = run.Findings
		active := activeRuleSet(engine.catalog, request.Selection, snapshot.Target)
		if active["capability-video-availability"] {
			findings = append(findings, capabilityAvailabilityFindings(snapshot, request.GopdsdkFloor, request.PlaydateSDK)...)
		}
		workspaceFindings, workspaceErr := workspaceFindings(ctx, snapshot, active)
		if workspaceErr != nil {
			return canceledResult(workspaceErr)
		}
		findings = append(findings, workspaceFindings...)
		var changed []string
		if len(request.ChangedFiles) != 0 {
			var changedErr error
			changed, changedErr = normalizeChangedFiles(request.ChangedFiles)
			if changedErr != nil {
				return IncrementalResult{}, changedErr
			}
		}
		generated := request.Generated
		if generated == "" {
			generated = GeneratedExclude
		}
		findings, err = filterSourceFindings(snapshot, findings, generated, changedFileSet(changed))
		if err != nil {
			return IncrementalResult{}, err
		}
		if err := applyInlineSuppressions(snapshot, engine.catalog, findings, reportingSourcePaths(snapshot, generated, changedFileSet(changed)), active); err != nil {
			return IncrementalResult{}, err
		}
		sortFindings(findings)
	}
	if err := engine.current(workspace, request.Version); err != nil {
		return IncrementalResult{}, err
	}
	invalidated := engine.store(key, snapshot, findings, loads)
	progress(request.Progress, "complete", 3, 3)
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	duration := time.Since(started)
	metrics := IncrementalMetrics{IncrementalTime: duration, PeakMemoryBytes: memory.Alloc, CacheMisses: 1, InvalidatedItems: invalidated}
	if invalidated == 0 {
		metrics.ColdTime = duration
	}
	return IncrementalResult{Version: request.Version, Findings: cloneFindings(findings), LoadErrors: loads, Metrics: metrics}, nil
}

func progress(callback func(Progress), stage string, current, total int) {
	if callback != nil {
		callback(Progress{Stage: stage, Current: current, Total: total})
	}
}

func (engine *IncrementalEngine) current(workspace string, version uint64) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if version < engine.latest[workspace] {
		return ErrStaleAnalysis
	}
	return nil
}

func (engine *IncrementalEngine) cached(key string) *incrementalEntry {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	entry := engine.entries[key]
	if entry != nil {
		engine.clock++
		entry.used = engine.clock
	}
	return entry
}

func (engine *IncrementalEngine) store(key string, snapshot Snapshot, findings []Finding, loads []LoadError) int {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.clock++
	engine.entries[key] = &incrementalEntry{key: key, snapshot: snapshot, findings: cloneFindings(findings), loads: append([]LoadError(nil), loads...), used: engine.clock}
	invalidated := 0
	for len(engine.entries) > engine.limit {
		var oldest *incrementalEntry
		for _, candidate := range engine.entries {
			if oldest == nil || candidate.used < oldest.used || candidate.used == oldest.used && candidate.key < oldest.key {
				oldest = candidate
			}
		}
		delete(engine.entries, oldest.key)
		invalidated += len(oldest.snapshot.Packages)
	}
	return invalidated
}

func incrementalLoadErrors(snapshot Snapshot) []LoadError {
	var result []LoadError
	for _, pkg := range snapshot.Packages {
		result = append(result, pkg.Errors...)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Position < result[j].Position || result[i].Position == result[j].Position && result[i].Message < result[j].Message
	})
	return uniqueLoadErrors(result)
}

func cloneFindings(source []Finding) []Finding {
	result := append([]Finding(nil), source...)
	for index := range result {
		result[index].Related = append([]RelatedFinding(nil), source[index].Related...)
		if source[index].Fixes != nil {
			result[index].Fixes = make([]FindingFix, len(source[index].Fixes))
			for fixIndex := range source[index].Fixes {
				result[index].Fixes[fixIndex] = source[index].Fixes[fixIndex]
				result[index].Fixes[fixIndex].Edits = append([]FindingEdit(nil), source[index].Fixes[fixIndex].Edits...)
			}
		}
		if source[index].Suppression != nil {
			suppression := *source[index].Suppression
			result[index].Suppression = &suppression
		}
	}
	return result
}

func incrementalKey(ctx context.Context, root string, request IncrementalRequest) (string, error) {
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\x00%v\x00%v\x00%v\x00%s\x00%v\x00%s\x00%s\x00%v\x00%s\x00%s\x00", filepath.ToSlash(root), request.Patterns, request.BuildTags, request.Tests, request.Target, request.Selection, request.Schedule, request.Generated, request.ChangedFiles, request.GopdsdkFloor, request.PlaydateSDK)
	paths := make([]string, 0, len(request.Overlay))
	for name := range request.Overlay {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		fmt.Fprintf(hash, "overlay\x00%s\x00%x\x00", filepath.ToSlash(name), sha256.Sum256(request.Overlay[name]))
	}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			base := entry.Name()
			if name != root && (base == ".git" || base == ".cache" || base == "build") {
				return fs.SkipDir
			}
			return nil
		}
		relative, _ := filepath.Rel(root, name)
		if !incrementalInput(filepath.ToSlash(relative)) {
			return nil
		}
		data, readErr := os.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		fmt.Fprintf(hash, "file\x00%s\x00%x\x00", filepath.ToSlash(relative), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint analyzer workspace: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func incrementalInput(name string) bool {
	base := filepath.Base(name)
	return strings.HasSuffix(name, ".go") || base == "go.mod" || base == "go.sum" || base == RepositoryConfigName || base == "pdxinfo" || strings.Contains(name, "/resources/")
}
