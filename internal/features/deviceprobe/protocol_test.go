package deviceprobe

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/features/toolingprotocol"
)

func TestStructuredDeviceBuildResult(t *testing.T) {
	var output bytes.Buffer
	result := Result{Package: "example.com/game", Output: `C:\work\game.pdx`, Metrics: Metrics{StaticRAM: 1024, ELF: 2048, PDX: 4096}}
	if err := writeStructuredBuildResult(&output, result); err != nil {
		t.Fatal(err)
	}
	envelope, err := toolingprotocol.DecodeEnvelope(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var structured structuredBuildResult
	if err := json.Unmarshal(envelope.Result, &structured); err != nil {
		t.Fatal(err)
	}
	if envelope.Command != "build device" || structured.Schema != buildResultSchema || structured.Target != "device" || structured.Artifact != "C:/work/game.pdx" || structured.Metrics.PDXBytes != 4096 {
		t.Fatalf("structured result = %+v, envelope = %+v", structured, envelope)
	}
}

func TestStructuredDeviceBuildFailureAndPlanningProgress(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"build", "device", "--format", "json", "--progress", "--sdk", t.TempDir(), "."}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run succeeded")
	}
	envelope, decodeErr := toolingprotocol.DecodeEnvelope(stdout.Bytes())
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if envelope.OK || envelope.Failure == nil || envelope.Failure.Category != "build-failed" {
		t.Fatalf("failure envelope = %+v", envelope)
	}
	if !strings.Contains(stderr.String(), `"command":"build device"`) || !strings.Contains(stderr.String(), `"stage":"planning"`) {
		t.Fatalf("progress = %q", stderr.String())
	}
}

func TestStructuredDeviceRunResult(t *testing.T) {
	var output bytes.Buffer
	result := Result{Package: "example.com/game", Deploy: "installed", Run: "launched", Metrics: Metrics{StaticRAM: 10, ELF: 20, PDX: 30}}
	if err := writeStructuredRunResult(&output, result); err != nil {
		t.Fatal(err)
	}
	envelope, err := toolingprotocol.DecodeEnvelope(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var structured structuredRunResult
	if err := json.Unmarshal(envelope.Result, &structured); err != nil {
		t.Fatal(err)
	}
	if envelope.Command != "run device" || structured.Schema != runResultSchema || structured.Target != "device" || structured.Deployment != "installed" || structured.Execution != "launched" || structured.Metrics.ELFBytes != 20 {
		t.Fatalf("structured result = %+v, envelope = %+v", structured, envelope)
	}
}

func TestStructuredDeviceRunFailureCategories(t *testing.T) {
	for _, test := range []struct{ stage, category string }{{"planning", "build-failed"}, {"deployment", "deployment-failed"}, {"launch", "launch-failed"}} {
		var output bytes.Buffer
		err := writeStructuredRunFailure(&output, t.Context(), assertError("secret detail"), test.stage)
		if err == nil {
			t.Fatalf("stage %s returned no error", test.stage)
		}
		envelope, decodeErr := toolingprotocol.DecodeEnvelope(output.Bytes())
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if envelope.Failure == nil || envelope.Failure.Category != test.category || strings.Contains(output.String(), "secret") {
			t.Fatalf("stage %s envelope = %+v", test.stage, envelope)
		}
	}
}

type assertError string

func (err assertError) Error() string { return string(err) }
