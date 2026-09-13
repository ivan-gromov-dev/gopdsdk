// Package toolingprotocol defines the versioned CLI contract shared by IDE
// integrations. It contains presentation-independent protocol types; command
// implementations remain owned by their feature packages.
package toolingprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	// EnvelopeSchema identifies the stable command-result envelope.
	EnvelopeSchema = "gopdsdk-tooling-result/v1"
	// CapabilitiesSchema identifies the capability-query payload.
	CapabilitiesSchema = "gopdsdk-tooling-capabilities/v1"
)

// Envelope is the final result of one structured command. A successful result
// has Result and no Failure; a failed result has Failure and no Result.
type Envelope struct {
	Schema  string          `json:"schema"`
	Command string          `json:"command"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result,omitempty"`
	Failure *Failure        `json:"failure,omitempty"`
}

// Failure is a stable, presentation-independent failure record. Detail is
// intentionally optional and must not contain environment values or secrets.
type Failure struct {
	Category    string           `json:"category"`
	Detail      string           `json:"detail,omitempty"`
	Remediation *Remediation     `json:"remediation,omitempty"`
	Locations   []SourceLocation `json:"locations,omitempty"`
}

// SourceLocation is a one-based location in a workspace-relative source file.
type SourceLocation struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Remediation is a presentation-independent recovery action.
type Remediation struct {
	Action string `json:"action"`
	Value  string `json:"value,omitempty"`
}

// ProbeResult is shared by read-only Simulator, device-toolchain, and USB
// connection probes.
type ProbeResult struct {
	Schema        string       `json:"schema"`
	Probe         string       `json:"probe"`
	Discovered    bool         `json:"discovered"`
	Ready         bool         `json:"ready"`
	EvidenceLevel string       `json:"evidenceLevel"`
	Values        []ProbeValue `json:"values"`
}

type ProbeValue struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// DecodeProbeResult accepts additive fields and rejects unknown schema
// versions.
func DecodeProbeResult(data []byte) (ProbeResult, error) {
	var result ProbeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProbeResult{}, fmt.Errorf("decode probe result: %w", err)
	}
	if result.Schema != ProbeSchema {
		return ProbeResult{}, fmt.Errorf("unsupported probe result schema %q", result.Schema)
	}
	return result, nil
}

// ProbeSchema identifies the stable read-only probe result.
const ProbeSchema = "gopdsdk-probe/v1"

// ProgressSchema identifies structured progress events written as NDJSON.
const ProgressSchema = "gopdsdk-progress/v1"

// ProgressEvent reports one ordered, cancellable command stage.
type ProgressEvent struct {
	Schema   string `json:"schema"`
	Command  string `json:"command"`
	Sequence int    `json:"sequence"`
	Stage    string `json:"stage"`
}

// WriteProgress writes one compact JSON event followed by a newline.
func WriteProgress(out io.Writer, event ProgressEvent) error {
	if event.Schema != ProgressSchema || event.Command == "" || event.Sequence < 1 || event.Stage == "" {
		return fmt.Errorf("invalid progress event")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode progress event: %w", err)
	}
	if _, err := out.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write progress event: %w", err)
	}
	return nil
}

// Capabilities describes the structured subset supported by this binary.
type Capabilities struct {
	Schema                 string              `json:"schema"`
	EnvelopeOptionalFields []string            `json:"envelopeOptionalFields"`
	Commands               []CommandCapability `json:"commands"`
}

// CommandCapability describes one CLI command and its machine-readable
// contracts. Modes state whether a command supports text, JSON, or LSP.
type CommandCapability struct {
	Name           string   `json:"name"`
	Modes          []string `json:"modes"`
	ResultSchemas  []string `json:"resultSchemas"`
	EventSchemas   []string `json:"eventSchemas"`
	OptionalFields []string `json:"optionalFields"`
	Cancellable    bool     `json:"cancellable"`
}

// NewEnvelope wraps a result payload in the stable envelope.
func NewEnvelope(command string, result any) (Envelope, error) {
	if command == "" {
		return Envelope{}, fmt.Errorf("command is required")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return Envelope{}, fmt.Errorf("encode %s result: %w", command, err)
	}
	return Envelope{Schema: EnvelopeSchema, Command: command, OK: true, Result: data}, nil
}

// WriteResult writes one successful structured command result.
func WriteResult(out io.Writer, command string, result any) error {
	envelope, err := NewEnvelope(command, result)
	if err != nil {
		return err
	}
	data, err := envelope.JSON()
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		return fmt.Errorf("write %s result: %w", command, err)
	}
	return nil
}

// WriteFailure writes one structured command failure.
func WriteFailure(out io.Writer, command, category string, remediation *Remediation) error {
	return WriteFailureWithLocations(out, command, category, remediation, nil)
}

// WriteFailureWithLocations writes a structured failure with verified source
// coordinates. Callers must provide only workspace-relative paths.
func WriteFailureWithLocations(out io.Writer, command, category string, remediation *Remediation, locations []SourceLocation) error {
	envelope := Envelope{Schema: EnvelopeSchema, Command: command, OK: false, Failure: &Failure{Category: category, Remediation: remediation, Locations: locations}}
	data, err := envelope.JSON()
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		return fmt.Errorf("write %s failure: %w", command, err)
	}
	return nil
}

// SilentError preserves failure and exit semantics after a structured error
// has already been written, without duplicating it on stderr.
type SilentError struct{ Err error }

func (err *SilentError) Error() string { return err.Err.Error() }
func (err *SilentError) Unwrap() error { return err.Err }
func (err *SilentError) Silent() bool  { return true }
func (err *SilentError) ExitCode() int { return 2 }

// JSON returns deterministic UTF-8 JSON terminated by one newline.
func (envelope Envelope) JSON() ([]byte, error) {
	if err := validateEnvelope(envelope); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode tooling result: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeEnvelope accepts unknown fields for forward compatibility and rejects
// schema versions whose required semantics are unknown.
func DecodeEnvelope(data []byte) (Envelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode tooling result: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Envelope{}, fmt.Errorf("decode tooling result: trailing JSON value")
	}
	if err := validateEnvelope(envelope); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func validateEnvelope(envelope Envelope) error {
	if envelope.Schema != EnvelopeSchema {
		return fmt.Errorf("unsupported tooling result schema %q", envelope.Schema)
	}
	if envelope.Command == "" {
		return fmt.Errorf("tooling result command is required")
	}
	if envelope.Failure != nil && envelope.Failure.Category == "" {
		return fmt.Errorf("tooling result failure category is required")
	}
	if envelope.Failure != nil {
		for _, location := range envelope.Failure.Locations {
			if location.Path == "" || strings.HasPrefix(location.Path, "/") || strings.ContainsAny(location.Path, `\:`) || location.Path == ".." || strings.HasPrefix(location.Path, "../") || location.Line < 1 || location.Column < 1 {
				return fmt.Errorf("tooling result contains invalid source location")
			}
		}
	}
	if envelope.OK == (envelope.Failure != nil) || (envelope.OK && len(envelope.Result) == 0) || (!envelope.OK && len(envelope.Result) != 0) {
		return fmt.Errorf("tooling result success and failure fields are inconsistent")
	}
	return nil
}
