// Package analyzer owns static analysis of gopdsdk application contracts.
package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// RuleID is a stable diagnostic identifier. An identifier is never reused for
// different semantics.
type RuleID string

// RuleFamily groups rules by the contract they enforce.
type RuleFamily string

const (
	FamilyDevice      RuleFamily = "device"
	FamilyApplication RuleFamily = "application"
	FamilyCapability  RuleFamily = "capability"
	FamilyLifetime    RuleFamily = "lifetime"
	FamilyResult      RuleFamily = "result"
	FamilyOwnership   RuleFamily = "ownership"
	FamilyPerformance RuleFamily = "performance"
	FamilyWorkspace   RuleFamily = "workspace"
)

// Severity is the default impact of a proven contract violation.
type Severity string

const (
	SeverityError       Severity = "error"
	SeverityWarning     Severity = "warning"
	SeverityPerformance Severity = "performance"
	SeverityInformation Severity = "information"
)

// Confidence describes how much static evidence supports a finding.
type Confidence string

const (
	ConfidenceProven    Confidence = "proven"
	ConfidenceLikely    Confidence = "likely"
	ConfidenceHeuristic Confidence = "heuristic"
)

// Target is the program environment to which a contract applies.
type Target string

const (
	TargetShared    Target = "shared"
	TargetSimulator Target = "simulator"
	TargetDevice    Target = "device"
)

// Rule describes a diagnostic independently of its implementation. Analyzer
// implementations are attached to this metadata by the Step 1 registry.
type Rule struct {
	ID            RuleID     `json:"id"`
	Family        RuleFamily `json:"family"`
	Summary       string     `json:"summary"`
	Default       Severity   `json:"defaultSeverity"`
	Confidence    Confidence `json:"confidence"`
	Targets       []Target   `json:"targets"`
	ContractIDs   []string   `json:"contractIds"`
	Experimental  bool       `json:"experimental,omitempty"`
	Suppressible  bool       `json:"suppressible"`
	SafeFixPolicy string     `json:"safeFixPolicy"`
}

// ContractKind identifies one machine-checkable SDK contract shape.
type ContractKind string

const (
	ContractReplacement        ContractKind = "replacement"
	ContractOptionalCapability ContractKind = "optional-capability"
	ContractCallbackScope      ContractKind = "callback-scope"
	ContractOwnedHandle        ContractKind = "owned-handle"
	ContractBorrowedHandle     ContractKind = "borrowed-handle"
	ContractRetention          ContractKind = "retention"
	ContractClose              ContractKind = "close"
	ContractUpdateOnly         ContractKind = "update-only"
	ContractArgumentBound      ContractKind = "argument-bound"
	ContractAvailability       ContractKind = "availability"
)

// Contract is a normative SDK fact that a rule may consume. PositiveCase and
// NegativeCase are minimal intended corpus cases, not executable Go snippets.
type Contract struct {
	ID           string         `json:"id"`
	Kind         ContractKind   `json:"kind"`
	Subject      string         `json:"subject"`
	PublicAPI    []PublicSymbol `json:"publicApi"`
	GoSymbols    []GoSymbol     `json:"goSymbols,omitempty"`
	Targets      []Target       `json:"targets"`
	NormativeRef string         `json:"normativeRef"`
	Statement    string         `json:"statement"`
	PositiveCase string         `json:"positiveCase"`
	NegativeCase string         `json:"negativeCase"`
}

// GoSymbol identifies a language, built-in, standard-library, or cgo surface
// constrained by the device profile.
type GoSymbol struct {
	Package string       `json:"package"`
	Name    string       `json:"name"`
	Policy  SymbolPolicy `json:"policy"`
}

// SymbolPolicy states how a device-profile rule treats a Go surface.
type SymbolPolicy string

const (
	SymbolAllowed     SymbolPolicy = "allowed"
	SymbolForbidden   SymbolPolicy = "forbidden"
	SymbolReplacement SymbolPolicy = "replacement"
)

// PublicSymbol identifies one exported declaration that supplies evidence for
// a contract. Package is relative to the module root, such as "playdate" or
// "playdate/schedule". Member is empty for a type or function declaration.
type PublicSymbol struct {
	Package string `json:"package"`
	Name    string `json:"name"`
	Member  string `json:"member,omitempty"`
}

// Inventory is the versioned, machine-readable analyzer contract catalog.
type Inventory struct {
	Schema              string     `json:"schema"`
	Contracts           []Contract `json:"contracts"`
	Rules               []Rule     `json:"rules"`
	RuntimeMeasurements []string   `json:"runtimeMeasurements"`
}

var stableID = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)+$`)

// Validate rejects incomplete, ambiguous, or internally inconsistent catalog
// data before analyzer implementations can consume it.
func (inventory Inventory) Validate() error {
	if inventory.Schema != "gopdsdk-analyzer-contracts/v1" {
		return fmt.Errorf("unsupported inventory schema %q", inventory.Schema)
	}
	contracts := make(map[string]struct{}, len(inventory.Contracts))
	for _, contract := range inventory.Contracts {
		if !stableID.MatchString(contract.ID) {
			return fmt.Errorf("contract has invalid id %q", contract.ID)
		}
		if _, exists := contracts[contract.ID]; exists {
			return fmt.Errorf("duplicate contract id %q", contract.ID)
		}
		contracts[contract.ID] = struct{}{}
		if contract.Kind == "" || contract.Subject == "" || len(contract.PublicAPI)+len(contract.GoSymbols) == 0 || contract.NormativeRef == "" || contract.Statement == "" || contract.PositiveCase == "" || contract.NegativeCase == "" {
			return fmt.Errorf("contract %q is incomplete", contract.ID)
		}
		for _, symbol := range contract.PublicAPI {
			if symbol.Package == "" || symbol.Name == "" {
				return fmt.Errorf("contract %q has incomplete public API symbol", contract.ID)
			}
			if !strings.HasPrefix(symbol.Package, "playdate") || strings.Contains(symbol.Package, "..") {
				return fmt.Errorf("contract %q has invalid public API package %q", contract.ID, symbol.Package)
			}
		}
		for _, symbol := range contract.GoSymbols {
			if symbol.Package == "" || symbol.Name == "" || !symbol.Policy.valid() {
				return fmt.Errorf("contract %q has incomplete Go symbol", contract.ID)
			}
		}
		if !contract.Kind.valid() {
			return fmt.Errorf("contract %q has invalid kind %q", contract.ID, contract.Kind)
		}
		if !validDocumentationReference(contract.NormativeRef) {
			return fmt.Errorf("contract %q has invalid normative reference %q", contract.ID, contract.NormativeRef)
		}
		if err := validateTargets("contract "+contract.ID, contract.Targets); err != nil {
			return err
		}
	}
	rules := make(map[RuleID]struct{}, len(inventory.Rules))
	for _, rule := range inventory.Rules {
		if !stableID.MatchString(string(rule.ID)) {
			return fmt.Errorf("rule has invalid id %q", rule.ID)
		}
		if _, exists := rules[rule.ID]; exists {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		rules[rule.ID] = struct{}{}
		if rule.Family == "" || rule.Summary == "" || rule.Default == "" || rule.Confidence == "" || rule.SafeFixPolicy == "" {
			return fmt.Errorf("rule %q is incomplete", rule.ID)
		}
		if !rule.Family.valid() || !rule.Default.valid() || !rule.Confidence.valid() {
			return fmt.Errorf("rule %q has invalid classification", rule.ID)
		}
		if !strings.HasPrefix(string(rule.ID), string(rule.Family)+"-") {
			return fmt.Errorf("rule %q does not belong to family %q", rule.ID, rule.Family)
		}
		if err := validateTargets("rule "+string(rule.ID), rule.Targets); err != nil {
			return err
		}
		if len(rule.ContractIDs) == 0 {
			return fmt.Errorf("rule %q has no normative contract", rule.ID)
		}
		for _, id := range rule.ContractIDs {
			if _, exists := contracts[id]; !exists {
				return fmt.Errorf("rule %q references unknown contract %q", rule.ID, id)
			}
		}
	}
	if len(inventory.RuntimeMeasurements) == 0 {
		return fmt.Errorf("runtime measurement limits are empty")
	}
	return nil
}

func (policy SymbolPolicy) valid() bool {
	return policy == SymbolAllowed || policy == SymbolForbidden || policy == SymbolReplacement
}

func (kind ContractKind) valid() bool {
	switch kind {
	case ContractReplacement, ContractOptionalCapability, ContractCallbackScope, ContractOwnedHandle, ContractBorrowedHandle, ContractRetention, ContractClose, ContractUpdateOnly, ContractArgumentBound, ContractAvailability:
		return true
	default:
		return false
	}
}

func (family RuleFamily) valid() bool {
	switch family {
	case FamilyDevice, FamilyApplication, FamilyCapability, FamilyLifetime, FamilyResult, FamilyOwnership, FamilyPerformance, FamilyWorkspace:
		return true
	default:
		return false
	}
}

func (severity Severity) valid() bool {
	return severity == SeverityError || severity == SeverityWarning || severity == SeverityPerformance || severity == SeverityInformation
}

func (confidence Confidence) valid() bool {
	return confidence == ConfidenceProven || confidence == ConfidenceLikely || confidence == ConfidenceHeuristic
}

func validateTargets(owner string, targets []Target) error {
	if len(targets) == 0 {
		return fmt.Errorf("%s has no targets", owner)
	}
	seen := make(map[Target]struct{}, len(targets))
	for _, target := range targets {
		if target != TargetShared && target != TargetSimulator && target != TargetDevice {
			return fmt.Errorf("%s has invalid target %q", owner, target)
		}
		if _, exists := seen[target]; exists {
			return fmt.Errorf("%s repeats target %q", owner, target)
		}
		seen[target] = struct{}{}
	}
	return nil
}

// JSON returns a deterministic structured representation of the inventory.
func (inventory Inventory) JSON() ([]byte, error) {
	if err := inventory.Validate(); err != nil {
		return nil, err
	}
	copy := inventory
	copy.Contracts = append([]Contract(nil), inventory.Contracts...)
	copy.Rules = append([]Rule(nil), inventory.Rules...)
	copy.RuntimeMeasurements = append([]string(nil), inventory.RuntimeMeasurements...)
	sort.Slice(copy.Contracts, func(i, j int) bool { return copy.Contracts[i].ID < copy.Contracts[j].ID })
	sort.Slice(copy.Rules, func(i, j int) bool { return copy.Rules[i].ID < copy.Rules[j].ID })
	sort.Strings(copy.RuntimeMeasurements)
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode analyzer inventory: %w", err)
	}
	return append(data, '\n'), nil
}

func validDocumentationReference(reference string) bool {
	return strings.HasPrefix(reference, "API.md#") || strings.HasPrefix(reference, "docs/ANALYZER_ROADMAP.md#")
}
