package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

const BaselineResultSchema = "gopdsdk-baseline-result/v1"

type BaselineResult struct {
	Schema       string          `json:"schema"`
	Operation    string          `json:"operation"`
	Path         string          `json:"path"`
	Entries      int             `json:"entries"`
	StaleEntries []BaselineEntry `json:"staleEntries"`
}

// DecodeBaselineResult accepts additive fields and rejects unknown schema versions.
func DecodeBaselineResult(data []byte) (BaselineResult, error) {
	var result BaselineResult
	if err := json.Unmarshal(data, &result); err != nil {
		return BaselineResult{}, fmt.Errorf("decode baseline result: %w", err)
	}
	if result.Schema != BaselineResultSchema {
		return BaselineResult{}, fmt.Errorf("unsupported baseline result schema %q", result.Schema)
	}
	return result, nil
}

// RunRules exposes the exact analyzer inventory used by this binary.
func RunRules(ctx context.Context, args []string, stdout, stderr io.Writer, options CheckOptions) error {
	flags := flag.NewFlagSet("gopdsdk rules", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "json", "output format: json")
	if len(args) == 0 || args[0] != "rules" {
		return commandError(ExitConfiguration, errors.New("expected rules command"))
	}
	if err := flags.Parse(args[1:]); err != nil {
		return commandError(ExitConfiguration, err)
	}
	if *format != "json" || len(flags.Args()) != 0 {
		return commandError(ExitConfiguration, errors.New("usage: gopdsdk rules --format json"))
	}
	if err := ctx.Err(); err != nil {
		return commandError(ExitCanceled, err)
	}
	inventory := ContractInventory()
	if err := inventory.Validate(); err != nil {
		return commandError(ExitInternal, err)
	}
	if err := toolingprotocol.WriteResult(stdout, "rules", inventory); err != nil {
		return commandError(ExitInternal, err)
	}
	return nil
}

// RunBaseline owns deterministic creation, update, and validation of adoption baselines.
func RunBaseline(ctx context.Context, args []string, stdout, stderr io.Writer, options CheckOptions) error {
	if len(args) < 2 || args[0] != "baseline" {
		return commandError(ExitConfiguration, errors.New("usage: gopdsdk baseline <create|update|validate>"))
	}
	operation := args[1]
	if operation != "create" && operation != "update" && operation != "validate" {
		return commandError(ExitConfiguration, fmt.Errorf("unknown baseline operation %q", operation))
	}
	flags := flag.NewFlagSet("gopdsdk baseline "+operation, flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "path to a gopdsdk-check/v1 report")
	output := flags.String("output", ".gopdsdk-check-baseline.json", "baseline path")
	reason := flags.String("reason", "accepted migration debt", "reason for newly added entries")
	if err := flags.Parse(args[2:]); err != nil {
		return commandError(ExitConfiguration, err)
	}
	if len(flags.Args()) != 0 || *output == "" || (operation != "validate" && *input == "") || (operation != "validate" && strings.TrimSpace(*reason) == "") {
		return commandError(ExitConfiguration, fmt.Errorf("usage: gopdsdk baseline %s --input report.json [--output baseline.json] [--reason text]", operation))
	}
	if err := ctx.Err(); err != nil {
		return commandError(ExitCanceled, err)
	}
	root, err := filepath.Abs(options.ModuleRoot)
	if err != nil {
		return commandError(ExitConfiguration, fmt.Errorf("resolve module root: %w", err))
	}
	path, err := containedPath(root, *output)
	if err != nil {
		return commandError(ExitConfiguration, err)
	}
	var existing Baseline
	if operation == "update" || operation == "validate" {
		existing, err = loadBaseline(root, path, options.Catalog)
		if err != nil {
			return commandError(ExitConfiguration, err)
		}
	} else if _, err := os.Stat(path); err == nil {
		return commandError(ExitConfiguration, fmt.Errorf("baseline already exists: %s", filepath.ToSlash(*output)))
	} else if !errors.Is(err, os.ErrNotExist) {
		return commandError(ExitConfiguration, fmt.Errorf("inspect baseline: %w", err))
	}

	var report *Report
	if *input != "" {
		inputPath, pathErr := containedPath(root, *input)
		if pathErr != nil {
			return commandError(ExitConfiguration, pathErr)
		}
		data, readErr := os.ReadFile(inputPath)
		if readErr != nil {
			return commandError(ExitConfiguration, fmt.Errorf("read check report: %w", readErr))
		}
		decoded, decodeErr := DecodeReport(data)
		if decodeErr != nil {
			return commandError(ExitConfiguration, decodeErr)
		}
		report = &decoded
	}

	stale := []BaselineEntry{}
	resultBaseline := existing
	if report != nil {
		fresh, buildErr := baselineFromReport(*report, strings.TrimSpace(*reason), existing, options.Catalog)
		if buildErr != nil {
			return commandError(ExitConfiguration, buildErr)
		}
		stale = staleBaselineEntries(existing, fresh)
		resultBaseline = fresh
	}
	if operation != "validate" {
		if err := writeBaselineAtomic(path, resultBaseline, options.Catalog); err != nil {
			return commandError(ExitInternal, err)
		}
	}
	result := BaselineResult{Schema: BaselineResultSchema, Operation: operation, Path: filepath.ToSlash(path), Entries: len(resultBaseline.Entries), StaleEntries: stale}
	if err := toolingprotocol.WriteResult(stdout, "baseline "+operation, result); err != nil {
		return commandError(ExitInternal, err)
	}
	return nil
}

func containedPath(root, name string) (string, error) {
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must remain within the module root")
	}
	return path, nil
}

func baselineFromReport(report Report, reason string, existing Baseline, catalog RuleCatalog) (Baseline, error) {
	reasons := make(map[baselineKey]string, len(existing.Entries))
	for _, entry := range existing.Entries {
		reasons[entry.key()] = entry.Reason
	}
	baseline := Baseline{Schema: BaselineSchema, Entries: []BaselineEntry{}}
	for _, diagnostic := range report.Diagnostics {
		entry := BaselineEntry{Rule: diagnostic.Rule, Target: diagnostic.Target, Path: diagnostic.Primary.Path, Line: diagnostic.Primary.Start.Line, Column: diagnostic.Primary.Start.Column, Message: diagnostic.Message, Reason: reason}
		if previous := reasons[entry.key()]; previous != "" {
			entry.Reason = previous
		}
		baseline.Entries = append(baseline.Entries, entry)
	}
	sortBaselineEntries(baseline.Entries)
	if err := baseline.Validate(catalog); err != nil {
		return Baseline{}, err
	}
	return baseline, nil
}

func staleBaselineEntries(previous, current Baseline) []BaselineEntry {
	active := make(map[baselineKey]bool, len(current.Entries))
	for _, entry := range current.Entries {
		active[entry.key()] = true
	}
	stale := []BaselineEntry{}
	for _, entry := range previous.Entries {
		if !active[entry.key()] {
			stale = append(stale, entry)
		}
	}
	sortBaselineEntries(stale)
	return stale
}

func sortBaselineEntries(entries []BaselineEntry) {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Message < b.Message
	})
}

func writeBaselineAtomic(path string, baseline Baseline, catalog RuleCatalog) error {
	if err := baseline.Validate(catalog); err != nil {
		return err
	}
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create baseline directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".gopdsdk-baseline-*")
	if err != nil {
		return fmt.Errorf("create baseline staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err = io.Copy(temporary, bytes.NewReader(data)); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write baseline staging file: %w", err)
	}
	backupFile, err := os.CreateTemp(filepath.Dir(path), ".gopdsdk-baseline-backup-*")
	if err != nil {
		return fmt.Errorf("reserve baseline backup: %w", err)
	}
	backup := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		return fmt.Errorf("close baseline backup: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("prepare baseline backup: %w", err)
	}
	hadExisting := false
	if _, statErr := os.Stat(path); statErr == nil {
		if err := os.Rename(path, backup); err != nil {
			return fmt.Errorf("stage previous baseline: %w", err)
		}
		hadExisting = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect previous baseline: %w", statErr)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		if hadExisting {
			_ = os.Rename(backup, path)
		}
		return fmt.Errorf("replace baseline: %w", err)
	}
	if hadExisting {
		_ = os.Remove(backup)
	}
	committed = true
	return nil
}
