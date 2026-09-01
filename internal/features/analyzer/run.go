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

// DefaultCheckOptions builds the production analyzer composition. Rules are
// registered by their implementation steps; Step 2 intentionally starts with
// an empty implementation registry.
func DefaultCheckOptions() (CheckOptions, error) {
	catalog, err := NewRuleCatalog(ContractInventory())
	if err != nil {
		return CheckOptions{}, err
	}
	registry, err := NewRegistry(catalog)
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
	format := flags.String("format", "text", "output format: text or json")
	target := flags.String("target", "both", "analysis target: shared, simulator, device, or both")
	tags := flags.String("tags", "", "comma-separated Go build tags")
	tests := flags.Bool("tests", false, "include test variants")
	if err := flags.Parse(args[1:]); err != nil {
		return commandError(ExitConfiguration, err)
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
		patterns = []string{"./..."}
	}
	buildTags, err := checkBuildTags(*tags)
	if err != nil {
		return commandError(ExitConfiguration, err)
	}
	if options.AnalyzerVersion == "" || options.SDKVersion == "" {
		return commandError(ExitInternal, errors.New("check analyzer composition has no version"))
	}
	if options.ModuleRoot == "" {
		options.ModuleRoot = "."
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
		result, err := options.Registry.Run(ctx, snapshot, RuleSelection{})
		if err != nil {
			return classifyCheckError(err, ExitInternal)
		}
		findings = append(findings, result.Findings...)
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
	report, err := NewReport(options.Catalog, options.AnalyzerVersion, options.SDKVersion, findings)
	if err != nil {
		return commandError(ExitInternal, err)
	}
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
	if len(findings) != 0 {
		return commandError(ExitFindings, fmt.Errorf("found %d diagnostic(s)", len(findings)))
	}
	return nil
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
	seen := make(map[string]bool, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("check build tags contain an empty name")
		}
		if strings.ContainsAny(part, " \t\r\n") {
			return nil, fmt.Errorf("invalid check build tag %q", part)
		}
		if !seen[part] {
			seen[part] = true
			result = append(result, part)
		}
	}
	sort.Strings(result)
	return result, nil
}

func writeTextReport(out io.Writer, report Report) error {
	if len(report.Diagnostics) == 0 {
		_, err := fmt.Fprintln(out, "No findings.")
		return err
	}
	for _, diagnostic := range report.Diagnostics {
		if _, err := fmt.Fprintf(out, "%s:%d:%d: %s %s: %s [%s]\n", diagnostic.Primary.Path, diagnostic.Primary.Start.Line,
			diagnostic.Primary.Start.Column, diagnostic.Severity, diagnostic.Rule, diagnostic.Message, diagnostic.Target); err != nil {
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
