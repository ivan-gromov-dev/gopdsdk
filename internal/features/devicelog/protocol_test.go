package devicelog

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

func TestStructuredLogSeparatesMetadataAndPreservesBytes(t *testing.T) {
	contents := []byte{'l', 'o', 'g', '\n', 0xff, 0x00}
	var output bytes.Buffer
	if err := writeStructuredResult(&output, "crashlog", CrashLog, `F:\crashlog.txt`, contents); err != nil {
		t.Fatal(err)
	}
	envelope, err := toolingprotocol.DecodeEnvelope(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var result StructuredResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(result.Content.Data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Schema != ResultSchema || result.Metadata.Kind != string(CrashLog) || result.Metadata.Path != "F:/crashlog.txt" || result.Metadata.ByteCount != len(contents) || !bytes.Equal(decoded, contents) {
		t.Fatalf("result = %+v, decoded = %v", result, decoded)
	}
}

func TestStructuredLogFailuresAreTypedAndRedacted(t *testing.T) {
	for _, test := range []struct {
		err      error
		category string
	}{
		{ErrNoDevice, "not-connected"}, {ErrToolUnavailable, "tool-not-found"}, {ErrLogNotFound, "log-not-found"}, {ErrRetrieval, "retrieval-failed"},
	} {
		var output bytes.Buffer
		err := writeStructuredFailure(&output, t.Context(), "errorlog", errors.Join(test.err, errors.New("secret path")))
		if err == nil {
			t.Fatal("structured failure returned nil")
		}
		envelope, decodeErr := toolingprotocol.DecodeEnvelope(output.Bytes())
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if envelope.Failure == nil || envelope.Failure.Category != test.category || strings.Contains(output.String(), "secret") {
			t.Fatalf("failure = %+v, output = %s", envelope.Failure, output.String())
		}
	}
}

func TestLogProgressUsesReachedStages(t *testing.T) {
	var stages []string
	logProgress(func(stage string) { stages = append(stages, stage) }, "connection")
	logProgress(func(stage string) { stages = append(stages, stage) }, "retrieval")
	if strings.Join(stages, ",") != "connection,retrieval" {
		t.Fatalf("stages = %v", stages)
	}
}
