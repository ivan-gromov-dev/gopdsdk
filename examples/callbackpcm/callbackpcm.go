// Package callbackpcm demonstrates bounded update-thread PCM rendering.
package callbackpcm

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type game struct {
	source    playdate.PCMCallbackSource
	phase     float64
	frequency float64
	right     bool
	starve    bool
	dirty     bool
}

// New creates the callback PCM acceptance scene.
func New() playdate.Game { return &game{frequency: 220, dirty: true} }

func (g *game) Init(context playdate.Context) error {
	outputs, ok := context.(playdate.AudioOutputs)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	callbacks, ok := context.(playdate.CallbackAudio)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	channel, err := outputs.DefaultAudioChannel()
	if err != nil {
		return err
	}
	g.source, err = callbacks.NewPCMCallbackSource(channel, true, g.render)
	return err
}

func (g *game) render(left, right []int16) int {
	if g.starve {
		return 0
	}
	step := g.frequency / 44100
	for index := range left {
		sample := int16(9000)
		if g.phase >= .5 {
			sample = -sample
		}
		if g.right {
			left[index], right[index] = 0, sample
		} else {
			left[index], right[index] = sample, 0
		}
		g.phase += step
		if g.phase >= 1 {
			g.phase--
		}
	}
	return len(left)
}

func (g *game) Update(context playdate.Context) (bool, error) {
	input := context.Input()
	if input.Pressed.Has(playdate.ButtonLeft) {
		g.frequency = 220
		g.right = false
		g.dirty = true
	}
	if input.Pressed.Has(playdate.ButtonRight) {
		g.frequency = 660
		g.right = true
		g.dirty = true
	}
	if input.Pressed.Has(playdate.ButtonA) {
		g.starve = !g.starve
		g.dirty = true
	}
	if g.starve {
		// Keep the native underrun counter observable while the callback
		// deliberately declines to refill the ring.
		g.dirty = true
	}
	if !g.dirty {
		return false, nil
	}
	g.dirty = false
	underruns, err := g.source.UnderrunCount()
	if err != nil {
		return false, err
	}
	context.Clear()
	context.DrawText("P9.3 bounded PCM callback", 12, 20)
	context.DrawText("Left: 220 Hz in LEFT ear", 12, 54)
	context.DrawText("Right: 660 Hz in RIGHT ear", 12, 78)
	context.DrawText("A: deliberate starvation", 12, 102)
	context.DrawText("Starved: "+strconv.FormatBool(g.starve), 12, 136)
	context.DrawText("Native underruns: "+strconv.FormatUint(uint64(underruns), 10), 12, 160)
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecycleTerminate && g.source != nil {
		return g.source.Close()
	}
	return nil
}
