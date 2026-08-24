package schedule

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testContext struct {
	playdate.Context
	now   uint32
	draws int
}

func (context *testContext) CurrentTimeMilliseconds() uint32 { return context.now }
func (context *testContext) Clear()                          { context.draws++ }
func (context *testContext) DrawText(string, int, int)       { context.draws++ }

func TestIncrementalConsumerCompletesAcrossFrames(t *testing.T) {
	context := &testContext{}
	game := New().(*game)
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	for frame := 0; frame < activeFrames-1; frame++ {
		refresh, err := game.Update(context)
		if err != nil || !refresh {
			t.Fatalf("frame %d = %v, %v", frame, refresh, err)
		}
		context.now += 33
	}
	if game.finished || game.processed != totalItems-itemsPerStep*stepsPerFrame {
		t.Fatalf("completed early: finished %v, items %d", game.finished, game.processed)
	}
	if _, err := game.Update(context); err != nil {
		t.Fatal(err)
	}
	if !game.finished || !game.proofPassed() || game.processed != totalItems || game.frames != activeFrames {
		t.Fatalf("finished %v, items %d, frames %d", game.finished, game.processed, game.frames)
	}
	for range 5 {
		if _, err := game.Update(context); err != nil {
			t.Fatal(err)
		}
	}
	if game.executedSteps != totalSteps || game.completionFrame != activeFrames || !game.proofPassed() {
		t.Fatalf("proof did not persist: steps %d, completion frame %d", game.executedSteps, game.completionFrame)
	}
	if context.draws != (activeFrames+5)*9 {
		t.Fatalf("draw calls = %d", context.draws)
	}
	if err := game.HandleLifecycle(context, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
}
