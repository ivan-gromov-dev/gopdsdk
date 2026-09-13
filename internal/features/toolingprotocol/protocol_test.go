package toolingprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestEnvelopeRoundTripAndForwardCompatibility(t *testing.T) {
	original, err := NewEnvelope("capabilities", Capabilities{Schema: CapabilitiesSchema, Commands: []CommandCapability{}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := original.JSON()
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("\n}"), []byte(",\n  \"future\": true\n}"), 1)
	decoded, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	var originalResult, decodedResult Capabilities
	if err := json.Unmarshal(original.Result, &originalResult); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(decoded.Result, &decodedResult); err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != original.Schema || decoded.Command != original.Command || decoded.OK != original.OK || decodedResult.Schema != originalResult.Schema {
		t.Fatalf("round trip differs:\n%+v\n%+v", original, decoded)
	}
	unknown := bytes.Replace(data, []byte(EnvelopeSchema), []byte("gopdsdk-tooling-result/v2"), 1)
	if _, err := DecodeEnvelope(unknown); err == nil || !strings.Contains(err.Error(), "unsupported tooling result schema") {
		t.Fatalf("unknown schema error = %v", err)
	}
}

func TestCapabilitiesAreDeterministicAndDescribeStructuredSubset(t *testing.T) {
	var first, second bytes.Buffer
	if err := Run(context.Background(), []string{"capabilities"}, &first); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"capabilities"}, &second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) || !bytes.HasSuffix(first.Bytes(), []byte("\n")) {
		t.Fatalf("capability output is not deterministic: %q / %q", first.Bytes(), second.Bytes())
	}
	envelope, err := DecodeEnvelope(first.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var capabilities Capabilities
	if err := json.Unmarshal(envelope.Result, &capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.Schema != CapabilitiesSchema || len(capabilities.Commands) == 0 {
		t.Fatalf("incomplete capabilities: %+v", capabilities)
	}
	for index := 1; index < len(capabilities.Commands); index++ {
		if capabilities.Commands[index-1].Name >= capabilities.Commands[index].Name {
			t.Fatalf("commands are not sorted: %+v", capabilities.Commands)
		}
	}
}

func TestCapabilitiesRejectArgumentsAndCancellation(t *testing.T) {
	if err := Run(context.Background(), []string{"capabilities", "extra"}, &bytes.Buffer{}); err == nil {
		t.Fatal("Run accepted an extra argument")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Run(ctx, []string{"capabilities"}, &bytes.Buffer{}); err == nil {
		t.Fatal("Run ignored cancellation")
	}
}
