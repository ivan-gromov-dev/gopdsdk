// Package wavetable demonstrates a compact P9.3 wavetable modulation graph.
package wavetable

import (
	"encoding/binary"
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type game struct {
	sample            playdate.AudioSample
	synth             playdate.Synth
	channel           playdate.AudioChannel
	filter            playdate.OnePoleFilter
	filterLFO, panLFO playdate.LFO
	clock             playdate.AudioClock
	automatic         bool
	parameter         float32
	dirty             bool
}

// New creates the wavetable acceptance scene.
func New() playdate.Game { return &game{automatic: true, dirty: true} }

func (g *game) Init(context playdate.Context) error {
	samples, sok := context.(playdate.AudioSamples)
	synths, yok := context.(playdate.Synthesizers)
	channels, cok := context.(playdate.AudioChannels)
	effects, eok := context.(playdate.AudioEffects)
	clock, tok := context.(playdate.AudioClock)
	if !sok || !yok || !cok || !eok || !tok {
		return playdate.ErrAudioUnavailable
	}
	g.clock = clock
	data := sawTable(256)
	var err error
	g.sample, err = samples.NewSampleFromData(data, playdate.Sound16BitMono, 44100)
	if err != nil {
		return err
	}
	g.synth, err = synths.NewSynth(playdate.WaveformSine)
	if err != nil {
		return err
	}
	if err = g.synth.SetWavetable(g.sample, 8, 1, 1); err != nil {
		return err
	}
	if err = g.synth.SetEnvelope(.01, .15, .7, .35); err != nil {
		return err
	}
	g.filterLFO, err = synths.NewLFO(playdate.LFOTypeSine)
	if err != nil {
		return err
	}
	_ = g.filterLFO.SetRate(.35)
	_ = g.filterLFO.SetCenter(0)
	_ = g.filterLFO.SetDepth(.75)
	g.panLFO, err = synths.NewLFO(playdate.LFOTypeTriangle)
	if err != nil {
		return err
	}
	_ = g.panLFO.SetRate(.2)
	_ = g.panLFO.SetCenter(0)
	_ = g.panLFO.SetDepth(1)
	g.filter, err = effects.NewOnePoleFilter()
	if err != nil {
		return err
	}
	if err = g.filter.SetParameterModulator(g.filterLFO); err != nil {
		return err
	}
	if err = g.filter.SetMix(1); err != nil {
		return err
	}
	g.channel, err = channels.NewAudioChannel()
	if err != nil {
		return err
	}
	if err = g.channel.AddSource(g.synth); err != nil {
		return err
	}
	if err = g.channel.AddEffect(g.filter); err != nil {
		return err
	}
	return g.channel.SetPanModulator(g.panLFO)
}

func (g *game) Update(context playdate.Context) (bool, error) {
	pressed := context.Input().Pressed
	if pressed.Has(playdate.ButtonA) {
		now, err := g.clock.CurrentAudioTime()
		if err != nil {
			return false, err
		}
		if err = g.synth.PlayMIDINote(48, .8, 2.5, now); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if pressed.Has(playdate.ButtonB) {
		g.automatic = !g.automatic
		if g.automatic {
			if err := g.filter.SetParameterModulator(g.filterLFO); err != nil {
				return false, err
			}
		} else {
			if err := g.filter.SetParameterModulator(nil); err != nil {
				return false, err
			}
			if err := g.filter.SetParameter(g.parameter); err != nil {
				return false, err
			}
		}
		g.dirty = true
	}
	if pressed.Has(playdate.ButtonLeft) || pressed.Has(playdate.ButtonUp) || pressed.Has(playdate.ButtonRight) {
		g.automatic = false
		if pressed.Has(playdate.ButtonLeft) {
			g.parameter = -.75
		} else if pressed.Has(playdate.ButtonRight) {
			g.parameter = .75
		} else {
			g.parameter = 0
		}
		if err := g.filter.SetParameterModulator(nil); err != nil {
			return false, err
		}
		if err := g.filter.SetParameter(g.parameter); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if !g.dirty {
		return false, nil
	}
	g.dirty = false
	context.Clear()
	context.DrawText("P9.3 wavetable", 12, 24)
	context.DrawText("A: play generated table", 12, 58)
	context.DrawText("L/U/R: low/off/high pass", 12, 88)
	context.DrawText("B: auto | "+filterMode(g.automatic, g.parameter), 12, 118)
	return true, nil
}
func (g *game) HandleLifecycle(_ playdate.Context, e playdate.LifecycleEvent) error {
	if e != playdate.LifecycleTerminate {
		return nil
	}
	return errors.Join(g.channel.Close(), g.filter.Close(), g.filterLFO.Close(), g.panLFO.Close(), g.synth.Close(), g.sample.Close())
}

func sawTable(size int) []byte {
	data := make([]byte, size*2)
	for i := 0; i < size; i++ {
		v := -32767 + i*65534/(size-1)
		binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(v)))
	}
	return data
}

func filterMode(auto bool, v float32) string {
	if auto {
		return "auto"
	}
	if v < 0 {
		return "low-pass -0.75"
	}
	if v > 0 {
		return "high-pass +0.75"
	}
	return "off 0"
}
