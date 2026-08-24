// Package audio exercises playback, dynamic music, and P9 output control.
package audio

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	effectAsset = "audio/effect"
	musicAsset  = "audio/music"
)

type game struct {
	effect           playdate.SamplePlayer
	effectCompletion playdate.CompletionPlayer
	music            playdate.FilePlayer
	musicRatePlayer  playdate.VariableRatePlayer
	musicFader       playdate.FadingPlayer
	audioClock       playdate.AudioClock
	outputs          playdate.AudioOutputs
	defaultChannel   playdate.AudioChannel
	outputState      playdate.AudioOutputState
	channel          playdate.AudioChannel
	delayChannel     playdate.AudioChannel
	synth            playdate.Synth
	sequenceSynth    playdate.Synth
	lfo              playdate.LFO
	envelope         playdate.Envelope
	control          playdate.ControlSignal
	instrument       playdate.Instrument
	track            playdate.SequenceTrack
	sequence         playdate.Sequence
	effects          []playdate.AudioEffect
	effectIndex      int
	delay            playdate.DelayLine
	synthesizers     playdate.Synthesizers
	waveform         playdate.Waveform
	lfoType          playdate.LFOType
	modulation       int
	transpose        float32
	channelVolume    float32
	sampleRate       float32
	musicRate        float32
	length           float32
	effectState      playdate.PlaybackState
	musicState       playdate.PlaybackState
	audioTime        uint32
	sampleFinished   uint32
	fadeFinished     uint32
	sequenceFinished uint32
	sequenceLength   uint32
	sequenceTime     uint32
	sequencePlaying  bool
	fading           bool
	pan              float32
	dirty            bool
	closed           bool
}

// New creates the audio acceptance game.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	samples, ok := context.(playdate.SamplePlayers)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	effect, err := samples.LoadSamplePlayer(effectAsset)
	if err != nil {
		return err
	}
	g.effect = effect
	g.sampleRate = 1
	g.musicRate = 1
	g.dirty = true
	g.length, err = effect.Length()
	if err != nil {
		return errors.Join(err, effect.Close())
	}
	music, err := context.LoadFilePlayer(musicAsset)
	if err != nil {
		return errors.Join(err, effect.Close())
	}
	g.music = music
	clock, ok := context.(playdate.AudioClock)
	if !ok {
		return errors.Join(playdate.ErrAudioUnavailable, g.close())
	}
	g.audioClock = clock
	musicRatePlayer, ok := music.(playdate.VariableRatePlayer)
	if !ok {
		return errors.Join(playdate.ErrAudioUnavailable, g.close())
	}
	g.musicRatePlayer = musicRatePlayer
	musicFader, ok := music.(playdate.FadingPlayer)
	if !ok {
		return errors.Join(playdate.ErrAudioUnavailable, g.close())
	}
	g.musicFader = musicFader
	effectCompletion, effectOK := effect.(playdate.CompletionPlayer)
	musicCompletion, musicOK := music.(playdate.CompletionPlayer)
	if !effectOK || !musicOK {
		return errors.Join(playdate.ErrAudioUnavailable, g.close())
	}
	g.effectCompletion = effectCompletion
	if err = g.armSampleCompletion(); err != nil {
		return errors.Join(err, g.close())
	}
	if err = musicCompletion.SetFinishCallback(func() { g.dirty = true }); err != nil {
		return errors.Join(err, g.close())
	}
	if err = effect.SetVolume(.8, .8); err != nil {
		return errors.Join(err, g.close())
	}
	if err = music.SetVolume(.35, .35); err != nil {
		return errors.Join(err, g.close())
	}
	channels, channelsOK := context.(playdate.AudioChannels)
	outputs, outputsOK := context.(playdate.AudioOutputs)
	synthesizers, synthsOK := context.(playdate.Synthesizers)
	if !channelsOK || !outputsOK || !synthsOK {
		return errors.Join(playdate.ErrAudioUnavailable, g.close())
	}
	g.outputs = outputs
	g.defaultChannel, err = outputs.DefaultAudioChannel()
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.outputState, err = outputs.AudioOutputState()
	if err != nil {
		return errors.Join(err, g.close())
	}
	if err = outputs.SetAudioOutputsActive(true, true); err != nil {
		return errors.Join(err, g.close())
	}
	g.channel, err = channels.NewAudioChannel()
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.synthesizers = synthesizers
	g.waveform = playdate.WaveformTriangle
	g.lfoType = playdate.LFOTypeSine
	g.modulation = 1
	g.channelVolume = .35
	g.synth, err = synthesizers.NewSynth(g.waveform)
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.lfo, err = synthesizers.NewLFO(g.lfoType)
	if err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.lfo.SetRate(3); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.lfo.SetCenter(.5); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.lfo.SetDepth(.5); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.synth.SetEnvelope(.01, .08, .55, .3); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.synth.SetAmplitudeModulator(g.lfo); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.channel.AddSource(g.synth); err != nil {
		return errors.Join(err, g.close())
	}
	g.envelope, err = synthesizers.NewEnvelope(.01, .12, .7, .25)
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.control, err = synthesizers.NewControlSignal()
	if err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.control.AddEvent(0, 0, false); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.control.AddEvent(6, 4, true); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.control.AddEvent(12, 0, true); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.channel.AddSource(g.effect); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.channel.AddSource(g.music); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.channel.SetVolume(g.channelVolume); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.initDynamicMusic(context); err != nil {
		return errors.Join(err, g.close())
	}
	return nil
}

func (g *game) initDynamicMusic(context playdate.Context) error {
	sequencers, sequenceOK := context.(playdate.Sequencers)
	effects, effectsOK := context.(playdate.AudioEffects)
	if !sequenceOK || !effectsOK {
		return nil
	}
	var err error
	g.instrument, err = sequencers.NewInstrument()
	if err != nil {
		return err
	}
	g.sequenceSynth, err = g.synthesizers.NewSynth(playdate.WaveformTriangle)
	if err != nil {
		return err
	}
	if err = g.sequenceSynth.SetEnvelope(.01, .08, .55, .25); err != nil {
		return err
	}
	if err = g.instrument.AddVoice(g.sequenceSynth, 0, 127, 0); err != nil {
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
	for step, note := range []uint8{48, 55, 60, 55} {
		if err = g.track.AddNote(uint32(step*4), 3, note, .45); err != nil {
			return err
		}
	}
	if err = g.track.AddControlEvent(1, 0, 0, false); err != nil {
		return err
	}
	if err = g.track.AddControlEvent(1, 12, 1, true); err != nil {
		return err
	}
	g.sequence, err = sequencers.NewSequence()
	if err != nil {
		return err
	}
	if err = g.sequence.SetTempo(8); err != nil {
		return err
	}
	if err = g.sequence.SetTrack(0, g.track); err != nil {
		return err
	}
	g.sequenceLength, err = g.sequence.Length()
	if err != nil {
		return err
	}
	if g.sequenceLength == 0 {
		return playdate.ErrAudioRoute
	}
	if err = g.playSequence(); err != nil {
		return err
	}
	filter, err := effects.NewTwoPoleFilter(playdate.FilterLowPass)
	if err != nil {
		return err
	}
	if err = filter.SetFrequency(700); err != nil {
		return err
	}
	if err = filter.SetMix(0); err != nil {
		return err
	}
	crusher, err := effects.NewBitCrusher()
	if err != nil {
		return err
	}
	if err = crusher.SetExponential(true); err != nil {
		return err
	}
	if err = crusher.SetDepth(.35); err != nil {
		return err
	}
	if err = crusher.SetDownsampling(.25); err != nil {
		return err
	}
	if err = crusher.SetMix(0); err != nil {
		return err
	}
	ring, err := effects.NewRingModulator()
	if err != nil {
		return err
	}
	if err = ring.SetFrequency(80); err != nil {
		return err
	}
	if err = ring.SetMix(0); err != nil {
		return err
	}
	overdrive, err := effects.NewOverdrive()
	if err != nil {
		return err
	}
	if err = overdrive.SetGain(4); err != nil {
		return err
	}
	if err = overdrive.SetLimit(.4); err != nil {
		return err
	}
	if err = overdrive.SetMix(0); err != nil {
		return err
	}
	g.delay, err = effects.NewDelayLine(11025, false)
	if err != nil {
		return err
	}
	if err = g.delay.SetFeedback(.1); err != nil {
		return err
	}
	if err = g.delay.SetMix(0); err != nil {
		return err
	}
	g.effects = []playdate.AudioEffect{filter, crusher, ring, overdrive, g.delay}
	for _, effect := range g.effects {
		if err = g.channel.AddEffect(effect); err != nil {
			return err
		}
	}
	tap, err := g.delay.AddTap(5512)
	if err != nil {
		return err
	}
	channels := context.(playdate.AudioChannels)
	g.delayChannel, err = channels.NewAudioChannel()
	if err != nil {
		return err
	}
	if err = g.delayChannel.SetVolume(0); err != nil {
		return err
	}
	if err = g.delayChannel.AddSource(tap); err != nil {
		return err
	}
	return g.applyEffectSelection()
}

func (g *game) playSequence() error {
	return g.sequence.Play(func() { g.sequenceFinished++; g.dirty = true })
}

func (g *game) applyEffectSelection() error {
	for _, effect := range g.effects {
		if err := effect.SetMix(0); err != nil {
			return err
		}
	}
	if err := g.delayChannel.SetVolume(0); err != nil {
		return err
	}
	if g.effectIndex == 4 {
		return g.delayChannel.SetVolume(.4)
	}
	return g.effects[g.effectIndex].SetMix(1)
}

func (g *game) Update(context playdate.Context) (bool, error) {
	outputState, err := g.outputs.AudioOutputState()
	if err != nil {
		return false, err
	}
	if outputState != g.outputState {
		g.outputState = outputState
		g.dirty = true
	}
	input := context.Input()
	if input.Pressed != 0 {
		g.audioTime, err = g.audioClock.CurrentAudioTime()
		if err != nil {
			return false, err
		}
	}
	buttons := input.Buttons | input.Pressed
	aHeld := buttons.Has(playdate.ButtonA)
	bHeld := buttons.Has(playdate.ButtonB)
	direction := input.Pressed & (playdate.ButtonLeft | playdate.ButtonRight | playdate.ButtonUp | playdate.ButtonDown)
	if input.Pressed.Has(playdate.ButtonA) && direction == 0 {
		if err := g.synth.PlayMIDINote(60, .8, -1, g.audioTime); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if input.Released.Has(playdate.ButtonA) {
		g.audioTime, err = g.audioClock.CurrentAudioTime()
		if err != nil {
			return false, err
		}
		if err = g.synth.NoteOff(g.audioTime); err != nil {
			return false, err
		}
	}
	if input.CrankDelta != 0 {
		g.pan = input.CrankAngle/180 - 1
		if err := g.channel.SetPan(g.pan); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if input.Pressed.Has(playdate.ButtonB) && direction == 0 {
		if err := g.toggleMusic(); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if direction != 0 {
		switch {
		case aHeld && bHeld && (direction.Has(playdate.ButtonLeft) || direction.Has(playdate.ButtonRight)):
			delta := 1
			if direction.Has(playdate.ButtonLeft) {
				delta = -1
			}
			g.waveform = playdate.Waveform((int(g.waveform) + 8 + delta) % 8)
			if err := g.synth.SetWaveform(g.waveform); err != nil {
				return false, err
			}
		case aHeld && (direction.Has(playdate.ButtonLeft) || direction.Has(playdate.ButtonRight)):
			delta := 1
			if direction.Has(playdate.ButtonLeft) {
				delta = -1
			}
			if err := g.changeLFO(delta); err != nil {
				return false, err
			}
		case bHeld && (direction.Has(playdate.ButtonLeft) || direction.Has(playdate.ButtonRight)):
			delta := 1
			if direction.Has(playdate.ButtonLeft) {
				delta = -1
			}
			g.effectIndex = (g.effectIndex + len(g.effects) + delta) % len(g.effects)
			if err := g.applyEffectSelection(); err != nil {
				return false, err
			}
		case aHeld && direction.Has(playdate.ButtonUp):
			if err := g.playSequence(); err != nil {
				return false, err
			}
		case aHeld && direction.Has(playdate.ButtonDown):
			if err := g.sequence.Stop(); err != nil {
				return false, err
			}
		case bHeld && (direction.Has(playdate.ButtonUp) || direction.Has(playdate.ButtonDown)):
			if direction.Has(playdate.ButtonUp) {
				if err := g.playSample(); err != nil {
					return false, err
				}
			} else {
				if err := g.music.Stop(); err != nil {
					return false, err
				}
			}
		case direction.Has(playdate.ButtonUp) || direction.Has(playdate.ButtonDown):
			delta := 1
			if direction.Has(playdate.ButtonDown) {
				delta = -1
			}
			g.modulation = (g.modulation + 5 + delta) % 5
			if err := g.applyModulation(); err != nil {
				return false, err
			}
		}
		g.dirty = true
	}
	effectState, err := g.effect.State()
	if err != nil {
		return false, err
	}
	musicState, err := g.music.State()
	if err != nil {
		return false, err
	}
	if effectState != g.effectState || musicState != g.musicState {
		g.effectState, g.musicState = effectState, musicState
		g.dirty = true
	}
	if g.sequence != nil {
		sequencePlaying, err := g.sequence.IsPlaying()
		if err != nil {
			return false, err
		}
		sequenceTime, err := g.sequence.Time()
		if err != nil {
			return false, err
		}
		if sequencePlaying != g.sequencePlaying || sequenceTime != g.sequenceTime {
			g.sequencePlaying, g.sequenceTime = sequencePlaying, sequenceTime
			g.dirty = true
		}
	}
	if !g.dirty {
		return false, nil
	}
	g.dirty = false
	context.Clear()
	context.DrawText("P9.1 output + full audio graph", 12, 8)
	context.DrawText("A synth | B music | B+Up sample", 12, 30)
	context.DrawText("A+B+L/R wave | U/D modulation", 12, 52)
	context.DrawText("A+L/R LFO | B+L/R effect", 12, 74)
	context.DrawText("Output H/M: "+outputName(g.outputState.Headphones)+"/"+outputName(g.outputState.HeadsetMicrophone), 12, 96)
	context.DrawText("Wave: "+waveformName(g.waveform)+"  LFO: "+lfoName(g.lfoType), 12, 126)
	context.DrawText("Mod: "+modulationName(g.modulation)+"  FX: "+effectName(g.effectIndex), 12, 148)
	context.DrawText("Seq: "+boolName(g.sequencePlaying)+" "+smallUint(g.sequenceTime)+"/"+smallUint(g.sequenceLength), 12, 170)
	context.DrawText("SFX/Music: "+stateName(effectState)+"/"+stateName(musicState), 12, 192)
	context.DrawText("Done S/F/Q: "+smallUint(g.sampleFinished)+"/"+smallUint(g.fadeFinished)+"/"+smallUint(g.sequenceFinished), 12, 214)
	return true, nil
}

func (g *game) toggleMusic() error {
	state, err := g.music.State()
	if err != nil {
		return err
	}
	if state == playdate.PlaybackStopped {
		g.fading = false
		if err = g.music.SetVolume(.35, .35); err != nil {
			return err
		}
		if err = g.channel.AddSource(g.music); err != nil {
			return err
		}
		if err = g.music.Play(); err != nil {
			return err
		}
		return nil
	}
	if !g.fading {
		g.fading = true
		return g.musicFader.FadeVolume(0, 0, 22050, func() { g.fadeFinished++; g.dirty = true })
	}
	g.fading = false
	return g.music.Stop()
}

func (g *game) playSample() error {
	offset := float32(0)
	if g.sampleRate < 0 {
		offset = g.length
	}
	if err := g.effect.SetOffset(offset); err != nil {
		return err
	}
	if err := g.armSampleCompletion(); err != nil {
		return err
	}
	return g.effect.PlayRepeated(1, g.sampleRate)
}

func (g *game) armSampleCompletion() error {
	return g.effectCompletion.SetFinishCallback(func() {
		g.sampleFinished++
		g.dirty = true
	})
}

func (g *game) applyModulation() error {
	if err := g.synth.SetAmplitudeModulator(nil); err != nil {
		return err
	}
	if err := g.synth.SetFrequencyModulator(nil); err != nil {
		return err
	}
	switch g.modulation {
	case 1:
		if err := g.lfo.SetCenter(.5); err != nil {
			return err
		}
		if err := g.lfo.SetDepth(.5); err != nil {
			return err
		}
		return g.synth.SetAmplitudeModulator(g.lfo)
	case 2:
		if err := g.lfo.SetCenter(0); err != nil {
			return err
		}
		if err := g.lfo.SetDepth(8); err != nil {
			return err
		}
		return g.synth.SetFrequencyModulator(g.lfo)
	case 3:
		return g.synth.SetAmplitudeModulator(g.envelope)
	case 4:
		return g.synth.SetFrequencyModulator(g.control)
	default:
		return nil
	}
}

func (g *game) changeLFO(delta int) error {
	if err := g.synth.SetAmplitudeModulator(nil); err != nil {
		return err
	}
	if err := g.synth.SetFrequencyModulator(nil); err != nil {
		return err
	}
	if err := g.lfo.Close(); err != nil {
		return err
	}
	g.lfoType = playdate.LFOType((int(g.lfoType) + 7 + delta) % 7)
	lfo, err := g.synthesizers.NewLFO(g.lfoType)
	if err != nil {
		return err
	}
	g.lfo = lfo
	if err = g.lfo.SetRate(3); err != nil {
		return err
	}
	if g.lfoType == playdate.LFOTypeArpeggiator {
		if err = g.lfo.SetArpeggiation([]float32{0, 4, 7, 12}); err != nil {
			return err
		}
		g.modulation = 2
	}
	return g.applyModulation()
}

func waveformName(value playdate.Waveform) string {
	return [...]string{"square", "triangle", "sine", "noise", "saw", "PO phase", "PO digital", "PO vosim"}[value]
}

func lfoName(value playdate.LFOType) string {
	return [...]string{"square", "triangle", "sine", "sample/hold", "saw up", "saw down", "arp"}[value]
}

func modulationName(value int) string {
	return [...]string{"none", "amp LFO", "freq LFO", "envelope", "control"}[value]
}

func effectName(value int) string {
	return [...]string{"filter", "crusher", "ring", "overdrive", "delay"}[value]
}

func boolName(value bool) string {
	if value {
		return "play"
	}
	return "stop"
}

func outputName(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func percent(value float32) string { return smallUint(uint32(value*100)) + "%" }

func signedInt(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buffer [12]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buffer[index] = '-'
	} else {
		index--
		buffer[index] = '+'
	}
	return string(buffer[index:])
}

func smallUint(value uint32) string {
	if value == 0 {
		return "0"
	}
	var buffer [10]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}

func nextRate(rate, delta float32) float32 {
	rate += delta
	if rate == 0 {
		if delta < 0 {
			return -.25
		}
		return .25
	}
	return rate
}

func rateName(rate float32) string {
	switch rate {
	case -2:
		return "-2.00x"
	case -1.75:
		return "-1.75x"
	case -1.5:
		return "-1.50x"
	case -1.25:
		return "-1.25x"
	case -1:
		return "-1.00x"
	case -.75:
		return "-0.75x"
	case -.5:
		return "-0.50x"
	case -.25:
		return "-0.25x"
	case .25:
		return "0.25x"
	case .5:
		return "0.50x"
	case .75:
		return "0.75x"
	case 1:
		return "1.00x"
	case 1.25:
		return "1.25x"
	case 1.5:
		return "1.50x"
	case 1.75:
		return "1.75x"
	case 2:
		return "2.00x"
	default:
		return "invalid"
	}
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecyclePause || event == playdate.LifecycleLock {
		return errors.Join(g.effect.Pause(), g.music.Pause())
	}
	if event == playdate.LifecycleResume || event == playdate.LifecycleUnlock {
		return errors.Join(g.effect.Resume(), g.music.Resume())
	}
	if event == playdate.LifecycleTerminate {
		return g.close()
	}
	return nil
}

func (g *game) close() error {
	if g.closed {
		return nil
	}
	g.closed = true
	var musicErr, effectErr, synthErr, lfoErr, envelopeErr, controlErr, channelErr, sequenceErr, trackErr, instrumentErr error
	if g.sequence != nil {
		sequenceErr = g.sequence.Close()
	}
	if g.track != nil {
		trackErr = g.track.Close()
	}
	if g.instrument != nil {
		instrumentErr = g.instrument.Close()
	}
	if g.sequenceSynth != nil {
		synthErr = errors.Join(synthErr, g.sequenceSynth.Close())
	}
	for _, effect := range g.effects {
		effectErr = errors.Join(effectErr, effect.Close())
	}
	if g.synth != nil {
		synthErr = g.synth.Close()
	}
	if g.lfo != nil {
		lfoErr = g.lfo.Close()
	}
	if g.envelope != nil {
		envelopeErr = g.envelope.Close()
	}
	if g.control != nil {
		controlErr = g.control.Close()
	}
	if g.channel != nil {
		channelErr = g.channel.Close()
	}
	if g.delayChannel != nil {
		channelErr = errors.Join(channelErr, g.delayChannel.Close())
	}
	if g.defaultChannel != nil {
		channelErr = errors.Join(channelErr, g.defaultChannel.Close())
	}
	if g.music != nil {
		musicErr = g.music.Close()
	}
	if g.effect != nil {
		effectErr = errors.Join(effectErr, g.effect.Close())
	}
	return errors.Join(musicErr, effectErr, synthErr, lfoErr, envelopeErr, controlErr, channelErr, sequenceErr, trackErr, instrumentErr)
}

func stateName(state playdate.PlaybackState) string {
	if state == playdate.PlaybackPlaying {
		return "playing"
	}
	if state == playdate.PlaybackPaused {
		return "paused"
	}
	return "stopped"
}
