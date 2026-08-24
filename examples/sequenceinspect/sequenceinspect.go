// Package sequenceinspect demonstrates P9.3 sequence and track introspection.
package sequenceinspect

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type game struct {
	synth                   playdate.Synth
	instrument              playdate.Instrument
	track                   playdate.SequenceTrack
	sequence                playdate.Sequence
	channel                 playdate.AudioChannel
	note                    playdate.SequenceNote
	trackCount              uint
	controlCount, polyphony int
	step, offset            int
	playing, pass, dirty    bool
}

// New creates the sequence-introspection acceptance scene.
func New() playdate.Game { return &game{dirty: true} }

func (g *game) Init(context playdate.Context) error {
	sequencers, sok := context.(playdate.Sequencers)
	synths, yok := context.(playdate.Synthesizers)
	channels, cok := context.(playdate.AudioChannels)
	if !sok || !yok || !cok {
		return playdate.ErrAudioUnavailable
	}
	var err error
	g.synth, err = synths.NewSynth(playdate.WaveformSquare)
	if err != nil {
		return err
	}
	if err = g.synth.SetEnvelope(.01, .08, .45, .2); err != nil {
		return err
	}
	g.instrument, err = sequencers.NewInstrument()
	if err != nil {
		return err
	}
	if err = g.instrument.AddVoice(g.synth, 0, 127, 0); err != nil {
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
	for i, n := range []uint8{48, 52, 55, 60} {
		if err = g.track.AddNote(uint32(i*4), 3, n, .55); err != nil {
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
	if err = g.sequence.SetTempo(6); err != nil {
		return err
	}
	if err = g.sequence.SetTrack(0, g.track); err != nil {
		return err
	}
	g.trackCount, err = g.sequence.TrackCount()
	if err != nil {
		return err
	}
	readTrack, err := g.sequence.Track(0)
	if err != nil {
		return err
	}
	if readTrack == nil {
		return playdate.ErrAudioRoute
	}
	var ok bool
	g.note, ok, err = readTrack.NoteAt(0)
	if err != nil {
		return err
	}
	if !ok {
		return playdate.ErrAudioRoute
	}
	index, err := readTrack.NoteIndexAtStep(0)
	if err != nil {
		return err
	}
	g.controlCount, err = readTrack.ControlSignalCount()
	if err != nil {
		return err
	}
	signal, err := readTrack.SignalForController(1, false)
	if err != nil {
		return err
	}
	g.polyphony, err = readTrack.Polyphony()
	if err != nil {
		return err
	}
	attached, err := readTrack.Instrument()
	if err != nil {
		return err
	}
	g.pass = g.trackCount == 1 && index == 0 && g.note.Step == 0 && g.note.Note == 48 && g.controlCount == 1 && signal != nil && attached == g.instrument
	if !g.pass {
		return playdate.ErrAudioRoute
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	pressed := context.Input().Pressed
	if pressed.Has(playdate.ButtonA) {
		if err := g.sequence.Play(func() { g.dirty = true }); err != nil {
			return false, err
		}
		g.dirty = true
	}
	if pressed.Has(playdate.ButtonB) {
		if err := g.sequence.AllNotesOff(); err != nil {
			return false, err
		}
		if err := g.sequence.Stop(); err != nil {
			return false, err
		}
		g.dirty = true
	}
	playing, err := g.sequence.IsPlaying()
	if err != nil {
		return false, err
	}
	step, offset, err := g.sequence.CurrentStep()
	if err != nil {
		return false, err
	}
	if playing != g.playing || step != g.step || offset != g.offset {
		g.playing, g.step, g.offset = playing, step, offset
		g.dirty = true
	}
	if !g.dirty {
		return false, nil
	}
	g.dirty = false
	context.Clear()
	context.DrawText("P9.3 sequence inspect PASS", 10, 16)
	context.DrawText("A: play | B: stop/all off", 10, 46)
	context.DrawText("Tracks/control/poly: "+small(g.trackCount)+"/"+small(uint(g.controlCount))+"/"+small(uint(g.polyphony)), 10, 78)
	context.DrawText("First note: 48 at step 0", 10, 108)
	context.DrawText("Current step/offset: "+signed(g.step)+"/"+signed(g.offset), 10, 138)
	context.DrawText("Playing: "+yes(g.playing), 10, 168)
	return true, nil
}
func (g *game) HandleLifecycle(_ playdate.Context, e playdate.LifecycleEvent) error {
	if e != playdate.LifecycleTerminate {
		return nil
	}
	return errors.Join(g.sequence.Close(), g.track.Close(), g.instrument.Close(), g.synth.Close(), g.channel.Close())
}
func small(v uint) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
func signed(v int) string {
	if v < 0 {
		return "-" + small(uint(-v))
	}
	return small(uint(v))
}
func yes(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
