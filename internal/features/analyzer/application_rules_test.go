package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
)

func TestExternalApplicationEntry(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "gopdsdk")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.CommandContext(context.Background(), "go", "build", "-buildvcs=false", "-o", binary, "./cmd/gopdsdk")
	build.Dir = repositoryRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	root := checkFixture(t, "package game\n")
	writeAnalyzerFixture(t, root, "pdxinfo", "name=Entry fixture\n")
	for _, target := range []string{"shared", "simulator", "device"} {
		command := exec.CommandContext(context.Background(), binary, "check", "--target", target, "--format", "json", "--rules", "application-entry", ".")
		command.Dir = root
		output, err := command.Output()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != ExitFindings {
			t.Fatalf("target %s: exit %v; %s", target, err, output)
		}
		var report Report
		if err := json.Unmarshal(output, &report); err != nil {
			t.Fatal(err)
		}
		if len(report.Diagnostics) != 1 || report.Diagnostics[0].Rule != "application-entry" || report.Diagnostics[0].Severity != SeverityError {
			t.Fatalf("report: %+v", report)
		}
	}
}

func TestApplicationRules(t *testing.T) {
	for _, test := range []struct {
		name, source string
		entry        bool
		want         []RuleID
	}{
		{"missing entry", "package game", true, []RuleID{"application-entry"}},
		{"library", "package game", false, nil},
		{"bad entry", `package game; func New(x int) int { return x }`, true, []RuleID{"application-entry"}},
		{"lifecycle value", applicationFixture + `func New() playdate.Game { return factory() }; func factory() playdate.Game { return game{} }; func (*game) HandleLifecycle(playdate.Context, playdate.LifecycleEvent) error { return nil }`, true, []RuleID{"application-lifecycle-shape"}},
		{"lifecycle pointer", applicationFixture + `func New() playdate.Game { return &game{} }; func (*game) HandleLifecycle(playdate.Context, playdate.LifecycleEvent) error { return nil }`, true, nil},
		{"lifecycle wrong signature", applicationFixture + `func New() playdate.Game { return game{} }; func (game) HandleLifecycle(playdate.Context) error { return nil }`, true, []RuleID{"application-lifecycle-shape"}},
		{"lifecycle unknown factory", applicationFixture + `var factory func() playdate.Game; func New() playdate.Game { return factory() }; func (*game) HandleLifecycle(playdate.Context, playdate.LifecycleEvent) error { return nil }`, true, nil},
		{"scheduler init helper", `package game
import("github.com/ivan-gromov-dev/gopdsdk/playdate"; "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule")
type game struct { s *schedule.Scheduler }
func (g *game) Init(playdate.Context) error { advance(g.s); return nil }
func (g *game) Update(playdate.Context) (bool,error) { return true,nil }
func advance(s *schedule.Scheduler) { s.Update() }
`, false, []RuleID{"application-scheduler-update-boundary"}},
		{"scheduler guarded helper", `package game
import("github.com/ivan-gromov-dev/gopdsdk/playdate"; "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule")
type game struct { s *schedule.Scheduler }
func (g *game) Init(playdate.Context) error { advance(g.s,false); return nil }
func (g *game) Update(playdate.Context) (bool,error) { advance(g.s,true); return true,nil }
func advance(s *schedule.Scheduler, update bool) { if update { s.Update() } }
`, false, nil},
		{"scheduler update valid", `package game
import("github.com/ivan-gromov-dev/gopdsdk/playdate"; "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule")
type game struct { s *schedule.Scheduler }
func (g *game) Init(playdate.Context) error { return nil }
func (g *game) Update(playdate.Context) (bool,error) { g.s.Update(); return true,nil }
`, false, nil},
		{"nested stencil helper", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func draw(c playdate.BitmapCompositor, b playdate.Bitmap) { c.WithStencil(b,false,func() error { return nested(c,b) }) }
func nested(c playdate.BitmapCompositor,b playdate.Bitmap) error { return c.WithStencil(b,false,func() error { return nil }) }
`, false, []RuleID{"application-nested-stencil"}},
		{"sprite close", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func register(s playdate.Sprite) { s.SetUpdateCallback(func(active playdate.Sprite) { active.Close() }) }
`, false, []RuleID{"application-sprite-callback-close"}},
		{"framebuffer escape", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved []byte
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,e:=f.Bytes(); saved=b; return e }) }
`, false, []RuleID{"lifetime-framebuffer-escape"}},
		{"framebuffer copy", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved []byte
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,e:=f.Bytes(); saved=append([]byte(nil),b...); return e }) }
`, false, nil},
		{"bitmap escape", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved playdate.BitmapData
func draw(g playdate.BitmapDataGraphics,b playdate.Bitmap) { g.WithBitmapData(b,func(d playdate.BitmapData) error { saved=d; return nil }) }
`, false, []RuleID{"lifetime-bitmap-data-escape"}},
		{"render escape", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved []int16
func register(g playdate.CallbackAudio,c playdate.AudioChannel) { g.NewPCMCallbackSource(c,true,func(left,right []int16) int { saved=left; return len(left) }) }
`, false, []RuleID{"lifetime-audio-render-buffer-escape"}},
		{"nested scheduler", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule"
func register(s *schedule.Scheduler) { s.Schedule(func() schedule.Action { s.Update(); return schedule.Complete() }) }
`, false, []RuleID{"application-nested-scheduler"}},
		{"termination leak", applicationFixture + `
func (game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error {
 if e != playdate.LifecycleTerminate { return nil }
 b,err := c.LoadBitmap("hero"); if err != nil { return err }; _ = b; return nil
}`, false, []RuleID{"application-termination-resource-leak"}},
		{"termination close", applicationFixture + `
func (game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error {
 if e != playdate.LifecycleTerminate { return nil }
 b,err := c.LoadBitmap("hero"); if err != nil { return err }; return b.Close()
}`, false, nil},
		{"termination other event", applicationFixture + `
func (game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error {
 if e != playdate.LifecyclePause { return nil }
 b,err := c.LoadBitmap("hero"); if err != nil { return err }; _ = b; return nil
}`, false, nil},
		{"termination correlated guard", applicationFixture + `
func (game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error {
 if e != playdate.LifecycleTerminate { return nil }
 b,err := c.LoadBitmap("hero"); if err != nil { return err }
 if err != nil { return nil }; return b.Close()
}`, false, nil},
		{"termination aggregate", applicationFixture + `
func cleanup(b playdate.Bitmap) { b.Close() }
func (game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error {
 if e != playdate.LifecycleTerminate { return nil }
 b,err := c.LoadBitmap("hero"); if err != nil { return err }; cleanup(b); return nil
}`, false, nil},
		{"microphone escape", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved playdate.MicrophoneSamples
func register(m playdate.Microphones) { m.StartMicrophoneRecording(0,func(samples playdate.MicrophoneSamples) bool { saved=samples; return true }) }
`, false, []RuleID{"lifetime-microphone-samples-escape"}},
		{"sprite helper", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func closeActive(s playdate.Sprite) { s.Close() }
func register(s playdate.Sprite) { s.SetUpdateCallback(func(active playdate.Sprite) { closeActive(active) }) }
`, false, []RuleID{"application-sprite-callback-close"}},
		{"embedded lifecycle", applicationFixture + `
type wrapper struct { game }
func (*wrapper) HandleLifecycle(playdate.Context, playdate.LifecycleEvent) error { return nil }
func New() playdate.Game { return &wrapper{} }
`, true, nil},
		{"generic factory", applicationFixture + `
type wrapper[T any] struct { game; value T }
func (*wrapper[T]) HandleLifecycle(playdate.Context, playdate.LifecycleEvent) error { return nil }
func New() playdate.Game { return makeGame[int]() }
func makeGame[T any]() playdate.Game { return &wrapper[T]{} }
`, true, nil},
		{"interface dispatch unknown", applicationFixture + `
type updater interface { Run() }
var hook updater
func New() playdate.Game { return game{} }
func (game) HandleLifecycle(playdate.Context, playdate.LifecycleEvent) error { hook.Run(); return nil }
`, true, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/game\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
			writeAnalyzerFixture(t, root, "game.go", test.source)
			if test.entry {
				writeAnalyzerFixture(t, root, "pdxinfo", "name=Fixture\n")
			}
			snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: TargetSimulator})
			if err != nil {
				t.Fatal(err)
			}
			if snapshotHasLoadErrors(snapshot) {
				t.Fatalf("load: %+v", snapshot.Packages)
			}
			options, err := DefaultCheckOptions()
			if err != nil {
				t.Fatal(err)
			}
			result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyApplication, FamilyLifetime}})
			if err != nil {
				t.Fatal(err)
			}
			var got []RuleID
			for _, finding := range result.Findings {
				got = append(got, finding.RuleID)
			}
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("findings=%+v, want %v", result.Findings, test.want)
			}
		})
	}
}

const applicationFixture = `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
type game struct{}
func (game) Init(playdate.Context) error { return nil }
func (game) Update(playdate.Context) (bool,error) { return true,nil }
`
