// Package generatorsynth demonstrates a polyphonic device-safe custom synth.
package generatorsynth

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

type game struct {
	synth      playdate.GeneratorSynth
	voiceSynth playdate.GeneratorSynth
	instrument playdate.Instrument
	track      playdate.SequenceTrack
	sequence   playdate.Sequence
	channel    playdate.AudioChannel
	clock      playdate.AudioClock
	phases     [8]uint32
	timbre     float32
	dirty      bool
}

// New creates the custom generator acceptance scene.
func New() playdate.Game { return &game{dirty: true} }

func (g *game) Init(context playdate.Context) error {
	generators, ok := context.(playdate.GeneratorSynthesizers)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	sequencers, ok := context.(playdate.Sequencers)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	channels, ok := context.(playdate.AudioChannels)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	var err error
	g.clock, ok = context.(playdate.AudioClock)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	g.synth, err = generators.NewGeneratorSynth(false, g.render)
	if err != nil {
		return err
	}
	if err = g.synth.SetParameter(1, g.timbre); err != nil {
		return err
	}
	if err = g.synth.SetEnvelope(.01, .08, .7, .2); err != nil {
		return err
	}
	g.voiceSynth, err = generators.NewGeneratorSynth(false, g.render)
	if err != nil {
		return err
	}
	if err = g.voiceSynth.SetParameter(1, g.timbre); err != nil {
		return err
	}
	if err = g.voiceSynth.SetEnvelope(.01, .08, .7, .2); err != nil {
		return err
	}
	g.instrument, err = sequencers.NewInstrument()
	if err != nil {
		return err
	}
	if err = g.instrument.AddVoice(g.voiceSynth, 0, 127, 0); err != nil {
		return err
	}
	g.channel, err = channels.NewAudioChannel()
	if err != nil {
		return err
	}
	if err = g.channel.AddSource(g.instrument); err != nil {
		return err
	}
	g.track, err = sequencers.NewSequenceTrack()
	if err != nil {
		return err
	}
	if err = g.track.SetInstrument(g.instrument); err != nil {
		return err
	}
	for _, note := range []uint8{60, 64, 67} {
		if err = g.track.AddNote(0, 3, note, .75); err != nil {
			return err
		}
	}
	g.sequence, err = sequencers.NewSequence()
	if err != nil {
		return err
	}
	if err = g.sequence.SetTempo(6); err != nil {
		return err
	}
	return g.sequence.SetTrack(0, g.track)
}

func (g *game) render(state playdate.GeneratorState, left, _ []int16) int {
	voice := int(state.Voice) % len(g.phases)
	phase := g.phases[voice]
	rate := state.Rate
	if rate == 0 {
		clear(left)
		return len(left)
	}
	drate := state.DeltaRate
	for index := range left {
		triangle := int32(phase >> 16)
		if triangle >= 32768 {
			triangle = 65535 - triangle
		}
		triangle = triangle*2 - 32767
		sample := triangle
		if state.Parameters[0] >= .5 {
			sample = -32767
			if phase < 1<<31 {
				sample = 32767
			}
		}
		left[index] = int16(float32(sample) * state.Velocity * .26)
		phase += rate
		rate = uint32(int64(rate) + int64(drate))
	}
	g.phases[voice] = phase
	return len(left)
}

func (g *game) Update(context playdate.Context) (bool, error) {
	input := context.Input()
	if input.Pressed.Has(playdate.ButtonA) {
		if err := g.sequence.Play(nil); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if input.Pressed.Has(playdate.ButtonB) {
		now, err := g.clock.CurrentAudioTime()
		if err != nil {
			return false, err
		}
		if err = g.synth.PlayMIDINote(72, .8, .7, now); err != nil {
			return false, err
		}
	}
	if input.Pressed.Has(playdate.ButtonLeft) || input.Pressed.Has(playdate.ButtonRight) {
		if input.Pressed.Has(playdate.ButtonLeft) {
			g.timbre = 0
		} else {
			g.timbre = 1
		}
		if err := g.synth.SetParameter(1, g.timbre); err != nil {
			return false, err
		}
		if err := g.voiceSynth.SetParameter(1, g.timbre); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if !g.dirty {
		return false, nil
	}
	g.dirty = false
	context.Clear()
	context.DrawText("P9.3 custom generator synth", 12, 20)
	context.DrawText("A: C-major chord (copied voices)", 12, 54)
	context.DrawText("B: root voice C5", 12, 78)
	context.DrawText("Left: triangle / Right: square", 12, 102)
	if g.timbre < .5 {
		context.DrawText("Current: triangle", 12, 136)
	} else {
		context.DrawText("Current: square", 12, 136)
	}
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event != playdate.LifecycleTerminate {
		return nil
	}
	if g.sequence != nil {
		_ = g.sequence.Close()
	}
	if g.channel != nil {
		_ = g.channel.Close()
	}
	if g.track != nil {
		_ = g.track.Close()
	}
	if g.instrument != nil {
		_ = g.instrument.Close()
	}
	if g.synth != nil {
		if err := g.synth.Close(); err != nil {
			return err
		}
	}
	if g.voiceSynth != nil {
		return g.voiceSynth.Close()
	}
	return nil
}
