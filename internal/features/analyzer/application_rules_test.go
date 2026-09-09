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
		{"termination reversed comparison", applicationFixture + `func(game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if playdate.LifecyclePause > e { _,err:=c.LoadBitmap("hero"); if err!=nil{return err}; return nil }; return nil }`, false, nil},
		{"lifecycle helper facts", `package game
import("github.com/ivan-gromov-dev/gopdsdk/playdate"; "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule")
type game struct{s *schedule.Scheduler}
func(g *game) Init(playdate.Context) error{return nil}
func(g *game) Update(playdate.Context)(bool,error){return true,nil}
func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent)error{if e==playdate.LifecycleTerminate{helper(g.s,true)};return nil}
func helper(s *schedule.Scheduler,run bool){if run{s.Update()}}
`, false, []RuleID{"application-scheduler-update-boundary"}},
		{"field contradictory guards", ownedFieldFixture + `func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.bitmap==nil { if g.bitmap!=nil { g.bitmap=nil } }; return nil }`, false, []RuleID{"application-termination-resource-leak"}},
		{"field private transfer", ownedFieldFixture + `var escaped playdate.Bitmap; func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.bitmap!=nil { escaped=g.bitmap; g.bitmap=nil }; return nil }`, false, nil},
		{"callback owner discarded", ownedCallbackFixture + `func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.source!=nil { g.source=nil }; return nil }`, false, []RuleID{"application-termination-resource-leak"}},
		{"callback owner closed", ownedCallbackFixture + `func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.source!=nil { return g.source.Close() }; return nil }`, false, nil},
		{"microphone aggregate cleanup", applicationFixture + `func(game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e!=playdate.LifecycleTerminate{return nil}; m:=c.(playdate.Microphones); _,err:=m.StartMicrophoneRecording(0,func(playdate.MicrophoneSamples)bool{return true}); if err!=nil{return err}; return nil }`, false, nil},
		{"owned field discarded", ownedFieldFixture + `func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.bitmap!=nil { g.bitmap=nil }; return nil }`, false, []RuleID{"application-termination-resource-leak"}},
		{"owned field retained", ownedFieldFixture + `func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.bitmap!=nil { return nil }; return nil }`, false, []RuleID{"application-termination-resource-leak"}},
		{"owned field closed", ownedFieldFixture + `func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.bitmap!=nil { return g.bitmap.Close() }; return nil }`, false, nil},
		{"owned field cleanup helper", ownedFieldFixture + `func cleanup(b playdate.Bitmap) { b.Close() }; func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.bitmap!=nil { cleanup(g.bitmap); g.bitmap=nil }; return nil }`, false, nil},
		{"field unknown origin", ownedFieldFixture + `func(g *game) Replace(b playdate.Bitmap) { g.bitmap=b }; func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecycleTerminate && g.bitmap!=nil { g.bitmap=nil }; return nil }`, false, nil},
		{"field other event", ownedFieldFixture + `func(g *game) HandleLifecycle(c playdate.Context,e playdate.LifecycleEvent) error { if e==playdate.LifecyclePause && g.bitmap!=nil { g.bitmap=nil }; return nil }`, false, nil},
		{"known helper true", `package game
import("github.com/ivan-gromov-dev/gopdsdk/playdate"; "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule")
type game struct{s *schedule.Scheduler}
func(g *game) Init(playdate.Context) error { helper(g.s,true); return nil }
func(g *game) Update(playdate.Context)(bool,error){return true,nil}
func helper(s *schedule.Scheduler,run bool){if run {s.Update()}}
`, false, []RuleID{"application-scheduler-update-boundary"}},
		{"native callback scheduler", `package game
import("github.com/ivan-gromov-dev/gopdsdk/playdate"; "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule")
func register(s playdate.Sprite, tasks *schedule.Scheduler) { s.SetDrawCallback(func(playdate.Sprite,playdate.Rect,playdate.Rect){tasks.Update()}) }
`, false, []RuleID{"application-scheduler-update-boundary"}},
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
		{"framebuffer return through helper", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved []byte
func retain(b []byte) []byte { return b }
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,_:=f.Bytes(); saved=retain(b); return nil }) }
`, false, []RuleID{"lifetime-framebuffer-escape"}},
		{"framebuffer map insertion", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved=map[string][]byte{}
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,_:=f.Bytes(); saved["frame"]=b; return nil }) }
`, false, []RuleID{"lifetime-framebuffer-escape"}},
		{"framebuffer closure capture", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var later func() int
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,_:=f.Bytes(); later=func()int{return len(b)}; return nil }) }
`, false, []RuleID{"lifetime-framebuffer-escape"}},
		{"framebuffer goroutine escape", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func use([]byte) {}
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,_:=f.Bytes(); go use(b); return nil }) }
`, false, []RuleID{"lifetime-framebuffer-escape"}},
		{"framebuffer subslice escape", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved []byte
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,_:=f.Bytes(); saved=b[:1]; return nil }) }
`, false, []RuleID{"lifetime-framebuffer-escape"}},
		{"framebuffer copy builtin", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
var saved=make([]byte,400*240/8)
func draw(g playdate.FramebufferGraphics) { g.WithFramebuffer(func(f playdate.Framebuffer) error { b,_:=f.Bytes(); copy(saved,b); return nil }) }
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

const ownedFieldFixture = `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
type game struct { bitmap playdate.Bitmap }
func(g *game) Init(c playdate.Context) error { b,err:=c.LoadBitmap("hero"); if err!=nil{return err}; g.bitmap=b; return nil }
func(g *game) Update(playdate.Context)(bool,error){return true,nil}
func New()playdate.Game{return &game{}}
`

const ownedCallbackFixture = `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
type game struct { source playdate.PCMCallbackSource }
func(g *game) Init(c playdate.Context) error { a:=c.(playdate.CallbackAudio); p,err:=a.NewPCMCallbackSource(nil,true,func(left,right []int16)int{return len(left)}); if err!=nil{return err}; g.source=p; return nil }
func(g *game) Update(playdate.Context)(bool,error){return true,nil}
func New()playdate.Game{return &game{}}
`
