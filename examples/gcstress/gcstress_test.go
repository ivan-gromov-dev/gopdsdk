package gcstress

import (
	"runtime"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/toolchainprofile"
	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testContext struct{ milliseconds uint32 }

func (testContext) Clear()                    {}
func (testContext) DrawText(string, int, int) {}
func (context *testContext) CurrentTimeMilliseconds() uint32 {
	context.milliseconds++
	return context.milliseconds
}
func (*testContext) Input() playdate.Input                                              { return playdate.Input{} }
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

type timingContext struct {
	milliseconds uint32
	step         uint32
}

func (*timingContext) Clear()                    {}
func (*timingContext) DrawText(string, int, int) {}
func (context *timingContext) CurrentTimeMilliseconds() uint32 {
	current := context.milliseconds
	context.milliseconds += context.step
	return current
}
func (*timingContext) Input() playdate.Input                                              { return playdate.Input{} }
func (*timingContext) LoadBitmap(string) (playdate.Bitmap, error)                         { return nil, nil }
func (*timingContext) LoadBitmapTable(string) (playdate.BitmapTable, error)               { return nil, nil }
func (*timingContext) NewBitmap(int, int) (playdate.Bitmap, error)                        { return nil, nil }
func (*timingContext) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (*timingContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*timingContext) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*timingContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*timingContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*timingContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (*timingContext) UpdateAndDrawSprites() {}

func TestUpdateKeepsBoundedLiveWindow(t *testing.T) {
	game := newGame(func() {}, func(*runtime.MemStats) {})
	context := &testContext{}
	const frames = retainedBlocks * 8
	for range frames {
		refresh, err := game.Update(context)
		if err != nil || !refresh {
			t.Fatalf("Update() = %v, %v; want true, nil", refresh, err)
		}
	}
	if game.frame != frames {
		t.Fatalf("frame = %d, want %d", game.frame, frames)
	}
	for index, block := range game.blocks {
		if len(block) != blockSize {
			t.Fatalf("blocks[%d] length = %d, want %d", index, len(block), blockSize)
		}
	}
}

func TestHeartbeatChangesAtFixedInterval(t *testing.T) {
	game := newGame(func() {}, func(*runtime.MemStats) {})
	context := &testContext{}
	for range heartbeatRate {
		if _, err := game.Update(context); err != nil {
			t.Fatal(err)
		}
	}
	if !game.heartbeat {
		t.Fatal("heartbeat did not change after heartbeatRate frames")
	}
}

func TestTelemetryReportsBoundedAfterHeapReuse(t *testing.T) {
	collections := 0
	game := newGame(func() { collections++ }, func(stats *runtime.MemStats) {
		stats.HeapAlloc = retainedBlocks * blockSize
		stats.TotalAlloc = heapSize + blockSize
		stats.Frees = 100
	})
	for range collectionRate {
		if _, err := game.Update(&testContext{}); err != nil {
			t.Fatal(err)
		}
	}
	if collections != 1 || !game.bounded || game.failed {
		t.Fatalf("collections/bounded/failed = %d/%v/%v, want 1/true/false", collections, game.bounded, game.failed)
	}
}

func TestTelemetryFailsWhenLiveHeapExceedsBudget(t *testing.T) {
	game := newGame(func() {}, func(stats *runtime.MemStats) {
		stats.HeapAlloc = maxLiveHeap + 1
	})
	for range collectionRate {
		if _, err := game.Update(&testContext{}); err != nil {
			t.Fatal(err)
		}
	}
	if !game.failed || game.bounded {
		t.Fatalf("failed/bounded = %v/%v, want true/false", game.failed, game.bounded)
	}
}

func TestTimingFailsWhenUpdateExceedsFrameBudget(t *testing.T) {
	game := newGame(func() {}, func(*runtime.MemStats) {})
	context := &timingContext{step: frameBudgetMS + 1}
	if _, err := game.Update(context); err != nil {
		t.Fatal(err)
	}
	if !game.timingFailed || game.maxUpdateMS <= frameBudgetMS {
		t.Fatalf("timingFailed/maxUpdateMS = %v/%d, want true/>%d", game.timingFailed, game.maxUpdateMS, frameBudgetMS)
	}
}

func TestTimingHandlesMillisecondCounterWrap(t *testing.T) {
	game := newGame(func() {}, func(*runtime.MemStats) {})
	context := &timingContext{milliseconds: ^uint32(0) - 1, step: 2}
	if _, err := game.Update(context); err != nil {
		t.Fatal(err)
	}
	if game.maxUpdateMS != 2 || game.timingFailed {
		t.Fatalf("maxUpdateMS/timingFailed = %d/%v, want 2/false", game.maxUpdateMS, game.timingFailed)
	}
}

func TestStatusReportsExactTimingMaxima(t *testing.T) {
	game := &game{bounded: true, heartbeat: true, maxUpdateMS: 7, maxGCMS: 3, elapsedMS: 4 * 1000}
	if got, want := game.statusText(), "GC ok U:7 G:3 S:4 +"; got != want {
		t.Fatalf("statusText() = %q, want %q", got, want)
	}
}

func TestRequiredSoakCompletesAfterSixtySecondsAcrossClockWrap(t *testing.T) {
	game := &game{bounded: true}
	start := ^uint32(0) - 100
	game.observeElapsed(start)
	game.observeElapsed(start + soakDurationMS - 1)
	if game.soakComplete {
		t.Fatal("soak completed before duration elapsed")
	}
	game.observeElapsed(start + soakDurationMS)
	if !game.soakComplete || game.elapsedMS != soakDurationMS {
		t.Fatalf("soakComplete/elapsedMS = %v/%d, want true/%d", game.soakComplete, game.elapsedMS, soakDurationMS)
	}
	if got := game.statusText(); got != "GC SOAK OK U:0 G:0 S:60 -" {
		t.Fatalf("statusText() = %q", got)
	}
}

func TestAcceptanceBudgetsMatchToolchainProfile(t *testing.T) {
	profile := toolchainprofile.Accepted()
	if frameBudgetMS != profile.Device.FrameBudgetMS {
		t.Fatalf("frame budget = %d, profile = %d", frameBudgetMS, profile.Device.FrameBudgetMS)
	}
	if soakDurationMS != profile.Device.RequiredSoakSeconds*1000 {
		t.Fatalf("required soak = %dms, profile = %ds", soakDurationMS, profile.Device.RequiredSoakSeconds)
	}
}
