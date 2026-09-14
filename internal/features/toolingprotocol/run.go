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
	capabilities := Capabilities{Schema: CapabilitiesSchema, EnvelopeOptionalFields: []string{"failure.detail", "failure.locations", "failure.remediation", "failure.remediation.value"}, Commands: []CommandCapability{
		{Name: "baseline", Modes: []string{"json"}, ResultSchemas: []string{"gopdsdk-baseline-result/v1"}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "build", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-build/v1"}, EventSchemas: []string{ProgressSchema}, OptionalFields: []string{"metrics"}, Cancellable: true},
		{Name: "capabilities", Modes: []string{"json"}, ResultSchemas: []string{CapabilitiesSchema}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: false},
		{Name: "check", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-check/v1"}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "crashlog", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-device-log/v1"}, EventSchemas: []string{ProgressSchema}, OptionalFields: []string{}, Cancellable: true},
		{Name: "device disk mount", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-device-disk/v1"}, EventSchemas: []string{ProgressSchema}, OptionalFields: []string{"mountPath"}, Cancellable: true},
		{Name: "device disk unmount", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-device-disk/v1"}, EventSchemas: []string{ProgressSchema}, OptionalFields: []string{"mountPath"}, Cancellable: true},
		{Name: "doctor", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-doctor/v1"}, EventSchemas: []string{}, OptionalFields: []string{"sdk", "tools[].version", "checks[].failureCategory", "checks[].remediation", "checks[].remediation.value"}, Cancellable: true},
		{Name: "errorlog", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-device-log/v1"}, EventSchemas: []string{ProgressSchema}, OptionalFields: []string{}, Cancellable: true},
		{Name: "init", Modes: []string{"text"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "lsp", Modes: []string{"lsp"}, ResultSchemas: []string{}, EventSchemas: []string{}, OptionalFields: []string{}, Cancellable: true},
		{Name: "probe", Modes: []string{"json", "text"}, ResultSchemas: []string{ProbeSchema}, EventSchemas: []string{}, OptionalFields: []string{"values"}, Cancellable: true},
		{Name: "rules", Modes: []string{"json"}, ResultSchemas: []string{"gopdsdk-analyzer-contracts/v1"}, EventSchemas: []string{}, OptionalFields: []string{"contracts[].goSymbols", "contracts[].publicApi[].member", "contracts[].publicApi[].minimumPlaydateSDK", "contracts[].publicApi[].sinceGopdsdk", "rules[].experimental"}, Cancellable: true},
		{Name: "run", Modes: []string{"json", "text"}, ResultSchemas: []string{"gopdsdk-run/v1"}, EventSchemas: []string{ProgressSchema}, OptionalFields: []string{"artifact", "deployment", "execution", "metrics", "pid"}, Cancellable: true},
	}}
	return WriteResult(out, "capabilities", capabilities)
}
