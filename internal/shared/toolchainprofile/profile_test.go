package toolchainprofile_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/buildplan"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/toolchainprofile"
)

func TestAcceptedProfileMatchesModuleAndDefaultDevicePlan(t *testing.T) {
	profile := toolchainprofile.Accepted()
	if profile.Schema != 1 || profile.Go == "" || profile.PlaydateSDK == "" || profile.TinyGo == "" || profile.ArmGCC == "" {
		t.Fatalf("accepted profile is incomplete: %#v", profile)
	}
	goMod, err := os.ReadFile(filepath.Join("..", "..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(goMod))
	goVersion := ""
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == "go" {
			goVersion = fields[index+1]
			break
		}
	}
	if goVersion != profile.Go {
		t.Fatalf("go.mod does not pin accepted Go %s", profile.Go)
	}
	plan, err := buildplan.New(buildplan.Device, ".", "sdk", "out")
	if err != nil {
		t.Fatal(err)
	}
	compile := strings.Join(plan.Commands[1].Args, " ")
	for _, want := range []string{"-gc " + profile.Device.Memory, "-scheduler " + profile.Device.Scheduler, "-panic " + profile.Device.Panic} {
		if !strings.Contains(compile, want) {
			t.Errorf("default device compile does not contain %q: %s", want, compile)
		}
	}
	link := strings.Join(plan.Commands[6].Args, " ")
	if want := "_heap_end=playdateRuntimeHeap+" + strconv.FormatUint(profile.Device.HeapBytes, 10); !strings.Contains(link, want) {
		t.Errorf("default device link does not contain %q: %s", want, link)
	}
	if profile.Device.RequiredSoakSeconds == 0 {
		t.Fatal("required hardware soak must be non-zero")
	}
}
