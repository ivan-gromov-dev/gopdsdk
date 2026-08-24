package audio

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type player struct {
	state          playdate.PlaybackState
	closed, paused bool
	plays          int
	volume         float32
	rate, offset   float32
	repeat         int
	finish         func()
	fade           func()
	fadeFrames     uint32
}

func (p *player) Play() error                            { p.state = playdate.PlaybackPlaying; p.plays++; return nil }
func (p *player) Stop() error                            { p.state = playdate.PlaybackStopped; p.paused = false; return nil }
func (p *player) SetVolume(left, _ float32) error        { p.volume = left; return nil }
func (p *player) Volume() (float32, float32, error)      { return p.volume, p.volume, nil }
func (p *player) State() (playdate.PlaybackState, error) { return p.state, nil }
func (p *player) Pause() error {
	if p.state == playdate.PlaybackPlaying {
		p.state = playdate.PlaybackPaused
		p.paused = true
	}
	return nil
}
func (p *player) Resume() error {
	if p.paused {
		p.state = playdate.PlaybackPlaying
		p.paused = false
	}
	return nil
}
func (p *player) Close() error { p.closed = true; p.state = playdate.PlaybackStopped; return nil }
func (p *player) PlayRepeated(repeat int, rate float32) error {
	p.repeat, p.rate, p.state = repeat, rate, playdate.PlaybackPlaying
	p.plays++
	return nil
}
func (*player) Length() (float32, error)                  { return 2.5, nil }
func (p *player) SetOffset(value float32) error           { p.offset = value; return nil }
func (p *player) Offset() (float32, error)                { return p.offset, nil }
func (p *player) SetRate(value float32) error             { p.rate = value; return nil }
func (p *player) Rate() (float32, error)                  { return p.rate, nil }
func (p *player) SetFinishCallback(callback func()) error { p.finish = callback; return nil }
func (p *player) FadeVolume(left, _ float32, frames uint32, callback func()) error {
	p.volume, p.fadeFrames, p.fade = left, frames, callback
	return nil
}
func (*player) SetWaveform(playdate.Waveform) error                    { return nil }
func (*player) SetEnvelope(float32, float32, float32, float32) error   { return nil }
func (*player) SetEnvelopeCurvature(float32) error                     { return nil }
func (*player) SetEnvelopeVelocitySensitivity(float32) error           { return nil }
func (*player) SetEnvelopeRateScaling(float32, uint8, uint8) error     { return nil }
func (*player) SetTranspose(float32) error                             { return nil }
func (*player) SetFrequencyModulator(playdate.Signal) error            { return nil }
func (*player) SetAmplitudeModulator(playdate.Signal) error            { return nil }
func (*player) SetWavetable(playdate.AudioSample, int, int, int) error { return nil }
func (*player) SetParameter(int, float32) error                        { return nil }
func (*player) SetParameterModulator(int, playdate.Signal) error       { return nil }
func (p *player) PlayMIDINote(float32, float32, float32, uint32) error {
	p.state = playdate.PlaybackPlaying
	p.plays++
	return nil
}
func (*player) NoteOff(uint32) error { return nil }

type signal struct{ closed bool }

func (*signal) Value() (float32, error)                    { return 0, nil }
func (*signal) SetScale(float32) error                     { return nil }
func (*signal) SetOffset(float32) error                    { return nil }
func (s *signal) Close() error                             { s.closed = true; return nil }
func (*signal) SetRate(float32) error                      { return nil }
func (*signal) SetPhase(float32) error                     { return nil }
func (*signal) SetCenter(float32) error                    { return nil }
func (*signal) SetDepth(float32) error                     { return nil }
func (*signal) SetRetrigger(bool) error                    { return nil }
func (*signal) SetArpeggiation([]float32) error            { return nil }
func (*signal) SetAttack(float32) error                    { return nil }
func (*signal) SetDecay(float32) error                     { return nil }
func (*signal) SetSustain(float32) error                   { return nil }
func (*signal) SetRelease(float32) error                   { return nil }
func (*signal) SetLegato(bool) error                       { return nil }
func (*signal) SetCurvature(float32) error                 { return nil }
func (*signal) SetVelocitySensitivity(float32) error       { return nil }
func (*signal) SetRateScaling(float32, uint8, uint8) error { return nil }
func (*signal) SetStartPhase(float32) error                { return nil }
func (*signal) SetRandomSeed(uint16) error                 { return nil }
func (*signal) SetGlobal(bool) error                       { return nil }
func (*signal) AddEvent(int, float32, bool) error          { return nil }
func (*signal) RemoveEvent(int) error                      { return nil }
func (*signal) ClearEvents() error                         { return nil }

type channel struct {
	source        playdate.AudioSource
	closed        bool
	adds, removes int
}

func (c *channel) Output() (playdate.AudioSource, error)  { return c.source, nil }
func (*channel) DryLevelSignal() (playdate.Signal, error) { return &signal{}, nil }
func (*channel) WetLevelSignal() (playdate.Signal, error) { return &signal{}, nil }
func (c *channel) AddSource(s playdate.AudioSource) error { c.source = s; c.adds++; return nil }
func (c *channel) RemoveSource(playdate.AudioSource) error {
	c.source = nil
	c.removes++
	return nil
}
func (*channel) AddEffect(playdate.AudioEffect) error     { return nil }
func (*channel) RemoveEffect(playdate.AudioEffect) error  { return nil }
func (*channel) SetVolume(float32) error                  { return nil }
func (*channel) Volume() (float32, error)                 { return 1, nil }
func (*channel) SetPan(float32) error                     { return nil }
func (*channel) SetPanModulator(playdate.Signal) error    { return nil }
func (*channel) SetVolumeModulator(playdate.Signal) error { return nil }
func (c *channel) Close() error                           { c.closed = true; return nil }

type context struct {
	effect         *player
	music          *player
	input          playdate.Input
	musicErr       error
	synth          *player
	lfo            *signal
	channel        *channel
	defaultChannel *channel
	outputs        [2]bool
	outputState    playdate.AudioOutputState
}

func (*context) CurrentTimeMilliseconds() uint32                                    { return 0 }
func (*context) CurrentAudioTime() (uint32, error)                                  { return 44100, nil }
func (c *context) Input() playdate.Input                                            { return c.input }
func (*context) Clear()                                                             {}
func (*context) DrawText(string, int, int)                                          {}
func (*context) LoadBitmap(string) (playdate.Bitmap, error)                         { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error)               { return nil, nil }
func (*context) NewBitmap(int, int) (playdate.Bitmap, error)                        { return nil, nil }
func (*context) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*context) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) { return nil, nil }
func (*context) UpdateAndDrawSprites()                                              {}
func (c *context) LoadSoundEffect(path string) (playdate.SoundEffect, error) {
	if path != effectAsset {
		panic(path)
	}
	c.effect = &player{}
	return c.effect, nil
}
func (c *context) LoadSamplePlayer(path string) (playdate.SamplePlayer, error) {
	if path != effectAsset {
		panic(path)
	}
	c.effect = &player{rate: 1}
	return c.effect, nil
}
func (c *context) LoadFilePlayer(path string) (playdate.FilePlayer, error) {
	if path != musicAsset {
		panic(path)
	}
	if c.musicErr != nil {
		return nil, c.musicErr
	}
	c.music = &player{}
	return c.music, nil
}
func (c *context) DefaultAudioChannel() (playdate.AudioChannel, error) {
	c.defaultChannel = &channel{}
	return c.defaultChannel, nil
}
func (c *context) AudioOutputState() (playdate.AudioOutputState, error) {
	return c.outputState, nil
}
func (c *context) SetAudioOutputsActive(headphones, speaker bool) error {
	c.outputs = [2]bool{headphones, speaker}
	return nil
}
func (c *context) NewAudioChannel() (playdate.AudioChannel, error) {
	c.channel = &channel{}
	return c.channel, nil
}
func (c *context) NewSynth(playdate.Waveform) (playdate.Synth, error) {
	c.synth = &player{}
	return c.synth, nil
}
func (c *context) NewLFO(playdate.LFOType) (playdate.LFO, error) {
	c.lfo = &signal{}
	return c.lfo, nil
}
func (*context) NewEnvelope(float32, float32, float32, float32) (playdate.Envelope, error) {
	return &signal{}, nil
}
func (*context) NewControlSignal() (playdate.ControlSignal, error) { return &signal{}, nil }

func TestRepeatedEffectMusicAndLifecycle(t *testing.T) {
	c := &context{}
	g := New().(*game)
	if err := g.Init(c); err != nil {
		t.Fatal(err)
	}
	c.outputState = playdate.AudioOutputState{Headphones: true, HeadsetMicrophone: true}
	refreshed, err := g.Update(c)
	if err != nil || !refreshed || g.outputState != c.outputState {
		t.Fatalf("output state refresh = %v, %#v, %v", refreshed, g.outputState, err)
	}
	for range 2 {
		c.input.Buttons = playdate.ButtonB
		c.input.Pressed = playdate.ButtonUp
		if _, err := g.Update(c); err != nil {
			t.Fatal(err)
		}
	}
	c.input.Buttons = 0
	if c.effect.plays != 2 {
		t.Fatalf("effect plays = %d", c.effect.plays)
	}
	if c.effect.repeat != 1 || c.effect.rate != 1 {
		t.Fatalf("sample repeat/rate = %d/%v", c.effect.repeat, c.effect.rate)
	}
	c.input.Pressed = playdate.ButtonA
	if _, err := g.Update(c); err != nil {
		t.Fatal(err)
	}
	if c.synth.plays != 1 {
		t.Fatalf("synth plays = %d", c.synth.plays)
	}
	c.input.Buttons = playdate.ButtonA | playdate.ButtonB
	c.input.Pressed = playdate.ButtonRight
	if _, err := g.Update(c); err != nil {
		t.Fatal(err)
	}
	if g.waveform != playdate.WaveformSine {
		t.Fatalf("waveform = %v", g.waveform)
	}
	c.input.Buttons = 0
	c.input.Pressed = playdate.ButtonB
	if _, err := g.Update(c); err != nil {
		t.Fatal(err)
	}
	if c.channel.removes != 0 || c.channel.source != c.music {
		t.Fatalf("music refresh removes = %d, source = %T", c.channel.removes, c.channel.source)
	}
	c.input.Pressed = playdate.ButtonB
	if _, err := g.Update(c); err != nil {
		t.Fatal(err)
	}
	if c.music.fadeFrames != 22050 || c.music.fade == nil {
		t.Fatalf("fade frames = %d, callback present = %t", c.music.fadeFrames, c.music.fade != nil)
	}
	c.music.fade()
	c.effect.finish()
	if g.fadeFinished != 1 || g.sampleFinished != 1 || g.audioTime != 44100 {
		t.Fatalf("callbacks/time = %d/%d/%d", g.fadeFinished, g.sampleFinished, g.audioTime)
	}
	for range 2 {
		c.input.Pressed = playdate.ButtonB
		if _, err := g.Update(c); err != nil {
			t.Fatal(err)
		}
	}
	if c.music.state != playdate.PlaybackPlaying || c.channel.source != c.music {
		t.Fatalf("music restart state/source = %v/%T", c.music.state, c.channel.source)
	}
	if err := g.HandleLifecycle(c, playdate.LifecyclePause); err != nil {
		t.Fatal(err)
	}
	if c.effect.state != playdate.PlaybackPaused || c.music.state != playdate.PlaybackPaused {
		t.Fatalf("paused states = %v/%v", c.effect.state, c.music.state)
	}
	if err := g.HandleLifecycle(c, playdate.LifecycleResume); err != nil {
		t.Fatal(err)
	}
	if c.effect.state != playdate.PlaybackPlaying || c.music.state != playdate.PlaybackPlaying {
		t.Fatalf("resumed states = %v/%v", c.effect.state, c.music.state)
	}
	if err := g.HandleLifecycle(c, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if !c.effect.closed || !c.music.closed {
		t.Fatal("players were not closed")
	}
	if !c.synth.closed || !g.lfo.(*signal).closed || !g.envelope.(*signal).closed || !g.control.(*signal).closed || !c.channel.closed {
		t.Fatal("music graph was not closed")
	}
}

func TestInitializationRollback(t *testing.T) {
	want := errors.New("music load")
	c := &context{musicErr: want}
	err := New().Init(c)
	if !errors.Is(err, want) || c.effect == nil || !c.effect.closed {
		t.Fatalf("Init() = %v, effect = %+v", err, c.effect)
	}
}
