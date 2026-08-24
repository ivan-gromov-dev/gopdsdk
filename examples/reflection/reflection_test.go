package reflection

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

func TestReflectionFixture(t *testing.T) {
	if got := reflectionFixture(); got != 56 {
		t.Fatalf("reflectionFixture() = %d, want 56", got)
	}
}

func TestGameCompletesSoakAcrossClockWrap(t *testing.T) {
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
	if !game.operationsOK || !game.soakComplete {
		t.Fatalf("operations/soak = %v/%v", game.operationsOK, game.soakComplete)
	}
}
