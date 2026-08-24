package lifecycleinput

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testContext struct {
	input playdate.Input
	lines []string
}

func (*testContext) Clear() {}
func (context *testContext) DrawText(text string, _, _ int) {
	context.lines = append(context.lines, text)
}
func (*testContext) CurrentTimeMilliseconds() uint32                                    { return 0 }
func (context *testContext) Input() playdate.Input                                      { return context.input }
func (*testContext) LoadBitmap(string) (playdate.Bitmap, error)                         { return nil, nil }
func (*testContext) LoadBitmapTable(string) (playdate.BitmapTable, error)               { return nil, nil }
func (*testContext) NewBitmap(int, int) (playdate.Bitmap, error)                        { return nil, nil }
func (*testContext) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (*testContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*testContext) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*testContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*testContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*testContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (*testContext) UpdateAndDrawSprites() {}

func TestGameDisplaysPortableSnapshot(t *testing.T) {
	probe := New().(*game)
	context := &testContext{input: playdate.Input{
		Buttons: playdate.ButtonA | playdate.ButtonLeft, Pressed: playdate.ButtonLeft,
		Released: playdate.ButtonB, Held: playdate.ButtonA,
		CrankAngle: 123.5, CrankDelta: -4.25, CrankDocked: false,
		CrankUndocked: true, DeltaSeconds: 0.0165,
	}}
	if err := probe.HandleLifecycle(context, playdate.LifecycleResume); err != nil {
		t.Fatal(err)
	}
	refresh, err := probe.Update(context)
	if err != nil || !refresh {
		t.Fatalf("Update() = %v, %v; want true, nil", refresh, err)
	}
	want := []string{
		"P1.1 lifecycle/input parity",
		"Lifecycle: resume",
		"Trace:R",
		"Count P/R:0/1 L/U:0/0 LP:0",
		"Buttons C:21 P:01 R:10 H:20",
		"Edges P*:01 R*:10",
		"Crank A:123.50 d:-4.25",
		"Docked:false +:false -:true",
		"Frame dt:16.50 ms",
		"Soak:RUN 0.02/60s",
	}
	if len(context.lines) != len(want) {
		t.Fatalf("lines = %v, want %v", context.lines, want)
	}
	for index := range want {
		if context.lines[index] != want[index] {
			t.Fatalf("lines[%d] = %q, want %q", index, context.lines[index], want[index])
		}
	}
}

func TestButtonEdgesRemainVisibleAfterTransitionFrame(t *testing.T) {
	probe := New().(*game)
	context := &testContext{input: playdate.Input{Pressed: playdate.ButtonA, Released: playdate.ButtonB}}
	if _, err := probe.Update(context); err != nil {
		t.Fatal(err)
	}
	context.input = playdate.Input{Buttons: playdate.ButtonA, Held: playdate.ButtonA}
	context.lines = nil
	if _, err := probe.Update(context); err != nil {
		t.Fatal(err)
	}
	if got := context.lines[5]; got != "Edges P*:20 R*:10" {
		t.Fatalf("latched edges = %q", got)
	}
}

func TestLifecycleTracePreservesOrderAndCounts(t *testing.T) {
	probe := New().(*game)
	context := &testContext{}
	events := []playdate.LifecycleEvent{
		playdate.LifecyclePause, playdate.LifecycleResume,
		playdate.LifecycleLock, playdate.LifecycleUnlock,
		playdate.LifecycleLowPower,
	}
	for _, event := range events {
		if err := probe.HandleLifecycle(context, event); err != nil {
			t.Fatal(err)
		}
	}
	if got := probe.lifecycleTraceLine(); got != "Trace:R>L>U>LP" {
		t.Fatalf("trace = %q, want last four events", got)
	}
	if got := probe.lifecycleCountLine(); got != "Count P/R:1/1 L/U:1/1 LP:1" {
		t.Fatalf("counts = %q", got)
	}
}

func TestRequiredSoakCompletesAtSixtySeconds(t *testing.T) {
	probe := New().(*game)
	context := &testContext{input: playdate.Input{DeltaSeconds: 30}}
	if _, err := probe.Update(context); err != nil {
		t.Fatal(err)
	}
	if probe.soakComplete {
		t.Fatal("soak completed before 60 seconds")
	}
	context.lines = nil
	if _, err := probe.Update(context); err != nil {
		t.Fatal(err)
	}
	if !probe.soakComplete || probe.elapsedSeconds != requiredSoakSeconds {
		t.Fatalf("soak complete/elapsed = %v/%v, want true/%d", probe.soakComplete, probe.elapsedSeconds, requiredSoakSeconds)
	}
	if got := context.lines[len(context.lines)-1]; got != "Soak:DONE 60.00/60s" {
		t.Fatalf("soak line = %q", got)
	}
}

func TestEveryLifecycleEventHasStableName(t *testing.T) {
	tests := []struct {
		event playdate.LifecycleEvent
		want  string
	}{
		{playdate.LifecyclePause, "pause"}, {playdate.LifecycleResume, "resume"},
		{playdate.LifecycleLock, "lock"}, {playdate.LifecycleUnlock, "unlock"},
		{playdate.LifecycleTerminate, "terminate"}, {playdate.LifecycleLowPower, "low-power"},
	}
	for _, test := range tests {
		if got := lifecycleName(test.event); got != test.want {
			t.Errorf("lifecycleName(%d) = %q, want %q", test.event, got, test.want)
		}
	}
}
