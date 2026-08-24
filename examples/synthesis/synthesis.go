// Package synthesis demonstrates P9.3 envelope shaping and native synthesis.
package synthesis

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

type game struct {
	synth     playdate.Synth
	lfo       playdate.LFO
	drySignal playdate.Signal
	wetSignal playdate.Signal
	synthBus  playdate.AudioChannel
	masterBus playdate.AudioChannel
	clock     playdate.AudioClock
	curvature float32
	dirty     bool
}

// New creates the synthesis acceptance scene.
func New() playdate.Game { return &game{curvature: 0, dirty: true} }

func (g *game) Init(context playdate.Context) error {
	synths, ok := context.(playdate.Synthesizers)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	clock, ok := context.(playdate.AudioClock)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	g.clock = clock
	channels, ok := context.(playdate.AudioChannels)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	var err error
	g.synthBus, err = channels.NewAudioChannel()
	if err != nil {
		return err
	}
	g.masterBus, err = channels.NewAudioChannel()
	if err != nil {
		return err
	}
	g.synth, err = synths.NewSynth(playdate.WaveformTriangle)
	if err != nil {
		return err
	}
	// Long attack and decay make the curvature difference intentionally obvious
	// in an audible acceptance scene.
	if err = g.synth.SetEnvelope(1.5, 1, .15, .6); err != nil {
		return err
	}
	if err = g.synth.SetEnvelopeCurvature(g.curvature); err != nil {
		return err
	}
	if err = g.synth.SetEnvelopeVelocitySensitivity(.6); err != nil {
		return err
	}
	if err = g.synth.SetEnvelopeRateScaling(.25, 36, 84); err != nil {
		return err
	}
	g.lfo, err = synths.NewLFO(playdate.LFOTypeSampleAndHold)
	if err != nil {
		return err
	}
	if err = g.lfo.SetRate(3); err != nil {
		return err
	}
	if err = g.lfo.SetDepth(.6); err != nil {
		return err
	}
	if err = g.lfo.SetStartPhase(.25); err != nil {
		return err
	}
	if err = g.lfo.SetRandomSeed(0x12_34); err != nil {
		return err
	}
	if err = g.lfo.SetGlobal(true); err != nil {
		return err
	}
	if err = g.synth.SetFrequencyModulator(g.lfo); err != nil {
		return err
	}
	if err = g.synthBus.AddSource(g.synth); err != nil {
		return err
	}
	output, err := g.synthBus.Output()
	if err != nil {
		return err
	}
	if err = g.masterBus.AddSource(output); err != nil {
		return err
	}
	g.drySignal, err = g.synthBus.DryLevelSignal()
	if err != nil {
		return err
	}
	g.wetSignal, err = g.synthBus.WetLevelSignal()
	if err != nil {
		return err
	}
	if err = g.masterBus.SetPanModulator(g.drySignal); err != nil {
		return err
	}
	if err = g.masterBus.SetVolumeModulator(g.wetSignal); err != nil {
		return err
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	input := context.Input()
	if input.Pressed.Has(playdate.ButtonA) {
		now, err := g.clock.CurrentAudioTime()
		if err != nil {
			return false, err
		}
		if err = g.synth.PlayMIDINote(60, .9, 3.5, now); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if input.Pressed.Has(playdate.ButtonLeft) || input.Pressed.Has(playdate.ButtonRight) {
		if input.Pressed.Has(playdate.ButtonLeft) {
			g.curvature -= .5
		} else {
			g.curvature += .5
		}
		if g.curvature < -1 {
			g.curvature = -1
		}
		if g.curvature > 1 {
			g.curvature = 1
		}
		if err := g.synth.SetEnvelopeCurvature(g.curvature); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if !g.dirty {
		return false, nil
	}
	g.dirty = false
	context.Clear()
	context.DrawText("P12.5 level signals", 12, 20)
	context.DrawText("Tap A after each change", 12, 52)
	context.DrawText("Left/Right: curve by 0.5", 12, 76)
	context.DrawText("Curve: "+curveName(g.curvature), 12, 112)
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecycleTerminate {
		if err := g.synth.Close(); err != nil {
			return err
		}
		if err := g.lfo.Close(); err != nil {
			return err
		}
		if err := g.synthBus.Close(); err != nil {
			return err
		}
		return g.masterBus.Close()
	}
	return nil
}

func curveName(v float32) string {
	value := int(v*10 + .5)
	if v < 0 {
		value = int(v*10 - .5)
	}
	if value == 0 {
		return "0.0 (linear)"
	}
	sign := "+"
	if value < 0 {
		sign = "-"
		value = -value
	}
	if value == 10 {
		return sign + "1.0"
	}
	return sign + "0." + string(rune('0'+value))
}
