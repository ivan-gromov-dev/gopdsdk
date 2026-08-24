package generatorsynth

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestRenderUsesRateVoiceAndParameter(t *testing.T) {
	g := &game{}
	left := make([]int16, 8)
	state := playdate.GeneratorState{Voice: 3, Velocity: 1, Rate: 1 << 28}
	if got := g.render(state, left, nil); got != len(left) || g.phases[3] == 0 {
		t.Fatalf("render = %d, phase %d", got, g.phases[3])
	}
	sine := left[1]
	g.phases[3] = 0
	state.Parameters[0] = 1
	g.render(state, left, nil)
	if left[1] == sine {
		t.Fatal("parameter did not change waveform")
	}
}

func TestRenderZeroRateIsSilent(t *testing.T) {
	g := &game{}
	left := make([]int16, 16)
	state := playdate.GeneratorState{Voice: 1, Note: 69, Velocity: 1}
	g.render(state, left, nil)
	if g.phases[1] != 0 {
		t.Fatalf("phase=%d", g.phases[1])
	}
	for index, sample := range left {
		if sample != 0 {
			t.Fatalf("left[%d]=%d", index, sample)
		}
	}
}
