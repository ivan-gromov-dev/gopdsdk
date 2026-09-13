// Package toolingprotocol defines the versioned CLI contract shared by IDE
// integrations. It contains presentation-independent protocol types; command
// implementations remain owned by their feature packages.
package toolingprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	Category string `json:"category"`
	Detail   string `json:"detail,omitempty"`
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
	if envelope.OK == (envelope.Failure != nil) || (envelope.OK && len(envelope.Result) == 0) || (!envelope.OK && len(envelope.Result) != 0) {
		return fmt.Errorf("tooling result success and failure fields are inconsistent")
	}
	return nil
}
