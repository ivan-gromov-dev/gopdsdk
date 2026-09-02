package analyzer

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"sort"
	"strings"
)

const (
	ExitSuccess       = 0
	ExitFindings      = 1
	ExitConfiguration = 2
	ExitPackageLoad   = 3
	ExitInternal      = 4
	ExitCanceled      = 130

	currentAnalyzerVersion = "v1"
)

// CommandError carries the stable process status for gopdsdk check.
type CommandError struct {
	Code int
	Err  error
}

func (err *CommandError) Error() string { return err.Err.Error() }
func (err *CommandError) Unwrap() error { return err.Err }
func (err *CommandError) ExitCode() int { return err.Code }

// CheckOptions supplies analyzer composition at the command boundary.
type CheckOptions struct {
	Catalog         RuleCatalog
	Registry        Registry
	AnalyzerVersion string
	SDKVersion      string
	ModuleRoot      string
}

// DefaultCheckOptions builds the production analyzer composition.
func DefaultCheckOptions() (CheckOptions, error) {
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		return CheckOptions{}, err
	}
	registrations := append(deviceRuleRegistrations(), applicationRuleRegistrations()...)
	registry, err := NewRegistry(catalog, registrations...)
	if err != nil {
		return CheckOptions{}, err
	}
	return CheckOptions{Catalog: catalog, Registry: registry, AnalyzerVersion: currentAnalyzerVersion, SDKVersion: sdkVersion(), ModuleRoot: "."}, nil
}

// RunCheck executes the gopdsdk check command.
func RunCheck(ctx context.Context, args []string, stdout, stderr io.Writer, options CheckOptions) error {
	if len(args) == 0 || args[0] != "check" {
		return commandError(ExitConfiguration, errors.New("expected check command"))
	}
	flags := flag.NewFlagSet("gopdsdk check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to repository check configuration")
	format := flags.String("format", "text", "output format: text or json")
	target := flags.String("target", "both", "analysis target: shared, simulator, device, or both")
	tags := flags.String("tags", "", "comma-separated Go build tags")
	tests := flags.Bool("tests", false, "include test variants")
	profile := flags.String("profile", "default", "rule profile: default, experimental, or deep")
	rulesFlag := flags.String("rules", "", "comma-separated rule selection")
	categoriesFlag := flags.String("categories", "", "comma-separated category selection")
	excludedFlag := flags.String("exclude-rules", "", "comma-separated rules to exclude")
	failOn := flags.String("fail-on", "warning", "exit threshold: error, warning, performance, information, or none")
	baselinePath := flags.String("baseline", "", "path to an adoption baseline")
	generated := flags.String("generated", "exclude", "generated sources: exclude or include")
	var changedValues repeatedFlag
	flags.Var(&changedValues, "changed-file", "module-relative changed file; may be repeated")
	var severityValues repeatedFlag
	flags.Var(&severityValues, "severity", "severity override selector=severity; may be repeated")
	if err := flags.Parse(args[1:]); err != nil {
		return commandError(ExitConfiguration, err)
	}
	if options.ModuleRoot == "" {
		options.ModuleRoot = "."
	}
	visited := make(map[string]bool)
	flags.Visit(func(value *flag.Flag) { visited[value.Name] = true })
	repositoryConfig, err := loadRepositoryConfig(options.ModuleRoot, *configPath, visited["config"])
	if err != nil {
		return commandError(ExitConfiguration, err)
	}
	if !visited["format"] && repositoryConfig.Format != nil {
		*format = *repositoryConfig.Format
	}
	if !visited["target"] && repositoryConfig.Target != nil {
		*target = *repositoryConfig.Target
	}
	if !visited["tests"] && repositoryConfig.Tests != nil {
		*tests = *repositoryConfig.Tests
	}
	if !visited["tags"] && repositoryConfig.BuildTags != nil {
		*tags = strings.Join(*repositoryConfig.BuildTags, ",")
	}
	if !visited["profile"] && repositoryConfig.Profile != nil {
		*profile = string(*repositoryConfig.Profile)
	}
	if !visited["rules"] && repositoryConfig.Rules != nil {
		*rulesFlag = joinRuleIDs(*repositoryConfig.Rules)
	}
	if !visited["categories"] && repositoryConfig.Categories != nil {
		*categoriesFlag = joinFamilies(*repositoryConfig.Categories)
	}
	if !visited["exclude-rules"] && repositoryConfig.ExcludedRules != nil {
		*excludedFlag = joinRuleIDs(*repositoryConfig.ExcludedRules)
	}
	if !visited["fail-on"] && repositoryConfig.FailOn != nil {
		*failOn = *repositoryConfig.FailOn
	}
	if !visited["baseline"] && repositoryConfig.Baseline != nil {
		*baselinePath = *repositoryConfig.Baseline
	}
	if !visited["generated"] && repositoryConfig.Generated != nil {
		*generated = string(*repositoryConfig.Generated)
	}
	changedFiles := append([]string(nil), changedValues...)
	if len(changedFiles) == 0 && repositoryConfig.ChangedFiles != nil {
		changedFiles = append(changedFiles, (*repositoryConfig.ChangedFiles)...)
	}
	if *generated != string(GeneratedExclude) && *generated != string(GeneratedInclude) {
		return commandError(ExitConfiguration, fmt.Errorf("invalid generated-source policy %q", *generated))
	}
	if len(changedFiles) != 0 {
		changedFiles, err = normalizeChangedFiles(changedFiles)
		if err != nil {
			return commandError(ExitConfiguration, err)
		}
	}
	changedSet := changedFileSet(changedFiles)
	severityOverrides := make(map[string]Severity, len(repositoryConfig.Severities)+len(severityValues))
	for selector, severity := range repositoryConfig.Severities {
		severityOverrides[selector] = severity
	}
	for _, value := range severityValues {
		selector, severity, ok := strings.Cut(value, "=")
		if !ok || selector == "" || !Severity(severity).valid() {
			return commandError(ExitConfiguration, fmt.Errorf("invalid severity override %q", value))
		}
		severityOverrides[selector] = Severity(severity)
	}
	if *format != "text" && *format != "json" {
		return commandError(ExitConfiguration, fmt.Errorf("invalid check format %q", *format))
	}
	targets, err := checkTargets(*target)
	if err != nil {
		return commandError(ExitConfiguration, err)
	}
	patterns := flags.Args()
	if len(patterns) == 0 {
		if repositoryConfig.Patterns != nil {
			patterns = append([]string(nil), (*repositoryConfig.Patterns)...)
		} else {
			patterns = []string{"./..."}
		}
	}
	buildTags, err := checkBuildTags(*tags)
	if err != nil {
		return commandError(ExitConfiguration, err)
	}
	if options.AnalyzerVersion == "" || options.SDKVersion == "" {
		return commandError(ExitInternal, errors.New("check analyzer composition has no version"))
	}
	policy := CheckPolicy{Profile: AnalysisProfile(*profile), Rules: splitRuleIDs(commaValues(*rulesFlag)), Categories: splitFamilies(commaValues(*categoriesFlag)),
		ExcludedRules: splitRuleIDs(commaValues(*excludedFlag)), SeverityOverrides: severityOverrides, FailOn: *failOn}
	selection, severityByRule, err := policy.Resolve(options.Catalog)
	if err != nil {
		return commandError(ExitConfiguration, err)
	}

	var findings []Finding
	var loadErrors []LoadError
	for _, selectedTarget := range targets {
		if err := ctx.Err(); err != nil {
			return commandError(ExitCanceled, err)
		}
		snapshot, err := LoadPackages(ctx, LoadConfig{ModuleRoot: options.ModuleRoot, Patterns: patterns, BuildTags: buildTags, Tests: *tests, Target: selectedTarget})
		if err != nil {
			return classifyCheckError(err, ExitConfiguration)
		}
		for _, pkg := range snapshot.Packages {
			loadErrors = append(loadErrors, pkg.Errors...)
		}
		if snapshotHasLoadErrors(snapshot) {
			continue
		}
		result, err := options.Registry.Run(ctx, snapshot, selection)
		if err != nil {
			return classifyCheckError(err, ExitInternal)
		}
		filtered, err := filterSourceFindings(snapshot, result.Findings, GeneratedPolicy(*generated), changedSet)
		if err != nil {
			return commandError(ExitInternal, err)
		}
		if err := applyInlineSuppressions(snapshot, options.Catalog, filtered, reportingSourcePaths(snapshot, GeneratedPolicy(*generated), changedSet), activeRuleSet(options.Catalog, selection, selectedTarget)); err != nil {
			return commandError(ExitConfiguration, err)
		}
		findings = append(findings, filtered...)
	}
	if len(loadErrors) != 0 {
		sort.Slice(loadErrors, func(i, j int) bool {
			if loadErrors[i].Position != loadErrors[j].Position {
				return loadErrors[i].Position < loadErrors[j].Position
			}
			if loadErrors[i].PackageID != loadErrors[j].PackageID {
				return loadErrors[i].PackageID < loadErrors[j].PackageID
			}
			return loadErrors[i].Message < loadErrors[j].Message
		})
		loadErrors = uniqueLoadErrors(loadErrors)
		return commandError(ExitPackageLoad, formatLoadErrors(loadErrors))
	}
	sortFindings(findings)
	if *baselinePath != "" {
		baseline, err := loadBaseline(options.ModuleRoot, *baselinePath, options.Catalog)
		if err != nil {
			return commandError(ExitConfiguration, err)
		}
		if err := applyBaseline(baseline, findings, changedSet); err != nil {
			return commandError(ExitConfiguration, err)
		}
	}
	report, err := NewReport(options.Catalog, options.AnalyzerVersion, options.SDKVersion, findings)
	if err != nil {
		return commandError(ExitInternal, err)
	}
	applySeverity(&report, severityByRule)
	if *format == "json" {
		data, err := report.JSON()
		if err == nil {
			_, err = stdout.Write(data)
		}
		if err != nil {
			return commandError(ExitInternal, err)
		}
	} else if err := writeTextReport(stdout, report); err != nil {
		return commandError(ExitInternal, err)
	}
	if reportFails(report, *failOn) {
		return commandError(ExitFindings, fmt.Errorf("found %d threshold-level diagnostic(s)", reportFailureCount(report, *failOn)))
	}
	return nil
}

func activeRuleSet(catalog RuleCatalog, selection RuleSelection, target Target) map[RuleID]bool {
	result := make(map[RuleID]bool)
	rules, _ := catalog.Select(selection)
	for _, rule := range rules {
		if containsTarget(rule.Targets, target) {
			result[rule.ID] = true
		}
	}
	return result
}

func snapshotHasLoadErrors(snapshot Snapshot) bool {
	for _, pkg := range snapshot.Packages {
		if len(pkg.Errors) != 0 {
			return true
		}
	}
	return false
}

type repeatedFlag []string

func (values *repeatedFlag) String() string         { return strings.Join(*values, ",") }
func (values *repeatedFlag) Set(value string) error { *values = append(*values, value); return nil }

func commaValues(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func joinRuleIDs(values []RuleID) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = string(value)
	}
	return strings.Join(parts, ",")
}
func joinFamilies(values []RuleFamily) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = string(value)
	}
	return strings.Join(parts, ",")
}

func uniqueLoadErrors(source []LoadError) []LoadError {
	result := source[:0]
	for _, item := range source {
		if len(result) != 0 {
			prior := result[len(result)-1]
			if prior.Position == item.Position && prior.PackageID == item.PackageID && prior.Kind == item.Kind && prior.Message == item.Message {
				continue
			}
		}
		result = append(result, item)
	}
	return result
}

func checkTargets(value string) ([]Target, error) {
	switch value {
	case "shared":
		return []Target{TargetShared}, nil
	case "simulator":
		return []Target{TargetSimulator}, nil
	case "device":
		return []Target{TargetDevice}, nil
	case "both":
		return []Target{TargetSimulator, TargetDevice}, nil
	default:
		return nil, fmt.Errorf("invalid check target %q", value)
	}
}

func checkBuildTags(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return normalizeBuildTags(parts)
}

func writeTextReport(out io.Writer, report Report) error {
	if len(report.Diagnostics) == 0 {
		_, err := fmt.Fprintln(out, "No findings.")
		return err
	}
	for _, diagnostic := range report.Diagnostics {
		suppressed := ""
		if diagnostic.Suppression != nil {
			suppressed = fmt.Sprintf(" (suppressed %s: %s)", diagnostic.Suppression.Kind, diagnostic.Suppression.Reason)
		}
		if _, err := fmt.Fprintf(out, "%s:%d:%d: %s %s: %s [%s]\n", diagnostic.Primary.Path, diagnostic.Primary.Start.Line,
			diagnostic.Primary.Start.Column, diagnostic.Severity, diagnostic.Rule, diagnostic.Message+suppressed, diagnostic.Target); err != nil {
			return err
		}
		for _, related := range diagnostic.Related {
			if _, err := fmt.Fprintf(out, "  related %s:%d:%d: %s\n", related.Range.Path, related.Range.Start.Line, related.Range.Start.Column, related.Message); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(out, "%d diagnostic(s).\n", len(report.Diagnostics))
	return err
}

func formatLoadErrors(loadErrors []LoadError) error {
	parts := make([]string, 0, len(loadErrors))
	for _, loadError := range loadErrors {
		location := loadError.Position
		if location == "" {
			location = loadError.PackageID
		}
		parts = append(parts, fmt.Sprintf("%s: %s", location, loadError.Message))
	}
	return errors.New("package load failed: " + strings.Join(parts, "; "))
}

func classifyCheckError(err error, fallback int) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return commandError(ExitCanceled, err)
	}
	return commandError(fallback, err)
}

func commandError(code int, err error) error { return &CommandError{Code: code, Err: err} }

func sdkVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}
