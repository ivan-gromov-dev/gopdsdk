package toolingprotocol

import (
	"context"
	"fmt"
	"io"
)

// Run writes the machine-readable capability query result.
func Run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 1 || args[0] != "capabilities" {
		return fmt.Errorf("usage: gopdsdk capabilities")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	capabilities := Capabilities{Schema: CapabilitiesSchema, EnvelopeOptionalFields: []string{"failure.detail"}, Commands: []CommandCapability{
		{Name: "build", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "capabilities", Modes: []string{"json"}, ResultSchemas: []string{CapabilitiesSchema}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: false},
		{Name: "check", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-check/v1"}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "crashlog", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "doctor", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "errorlog", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "init", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "lsp", Modes: []string{"lsp"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "probe", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "run", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
	}}
	envelope, err := NewEnvelope("capabilities", capabilities)
	if err != nil {
		return err
	}
	data, err := envelope.JSON()
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		return fmt.Errorf("write capabilities: %w", err)
	}
	return nil
}
