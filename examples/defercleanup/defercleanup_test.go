package defercleanup

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testContext struct {
	playdate.Context
	now uint32
}

func (context *testContext) CurrentTimeMilliseconds() uint32 { return context.now }
func (*testContext) Clear()                                  {}
func (*testContext) DrawText(string, int, int)               {}

func TestNormalReturnSemantics(t *testing.T) {
	if !verifySemantics() {
		t.Fatal("normal-return defer semantics failed")
	}
}

func TestRepeatedDeferCleansEveryResourceInLIFOFrame(t *testing.T) {
	cleaned := 0
	repeatedCleanup(repeatedCount, &cleaned)
	if cleaned != repeatedCount {
		t.Fatalf("cleanups = %d, want %d", cleaned, repeatedCount)
	}
}

func TestDurationFixture(t *testing.T) {
	got, err := durationFixture("1h2m3.004s")
	if err != nil || got != "1h2m3.004s" {
		t.Fatalf("durationFixture() = %q, %v", got, err)
	}
	if _, err := durationFixture("invalid"); err == nil {
		t.Fatal("durationFixture(invalid) error = nil")
	}
}

func TestGameRunsCleanupOnEveryFrameAndCompletesSoakAcrossWrap(t *testing.T) {
	context := &testContext{now: ^uint32(0) - 100}
	game := New().(*game)
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	if _, err := game.Update(context); err != nil {
		t.Fatal(err)
	}
	context.now += soakDurationMS
	if _, err := game.Update(context); err != nil {
		t.Fatal(err)
	}
	if !game.semanticsOK || !game.timeOK || !game.soakComplete || game.cleanups != 2*repeatedCount {
		t.Fatalf("semantics/time/soak/cleanups = %v/%v/%v/%d", game.semanticsOK, game.timeOK, game.soakComplete, game.cleanups)
	}
}
