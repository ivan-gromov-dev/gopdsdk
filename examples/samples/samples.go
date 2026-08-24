// Package samples exercises the P9.2 owned-sample and sample-player APIs.
package samples

import (
	"encoding/binary"
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const sampleRate = 44100

type game struct {
	sample     playdate.AudioSample
	player     playdate.SamplePlayer
	controls   playdate.SamplePlayerControls
	loopPlayer playdate.LoopCallbackPlayer
	loops      uint32
}

// New creates the samples acceptance game.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	samples, ok := context.(playdate.AudioSamples)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	players, ok := context.(playdate.SamplePlayerFactory)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	pcm := make([]byte, sampleRate/2*2)
	for frame := 0; frame < sampleRate/2; frame++ {
		value := int16(9000)
		if frame%(sampleRate/440) >= sampleRate/880 {
			value = -value
		}
		binary.LittleEndian.PutUint16(pcm[frame*2:], uint16(value))
	}
	sample, err := samples.NewSampleFromData(pcm, playdate.Sound16BitMono, sampleRate)
	if err != nil {
		return err
	}
	g.sample = sample
	view, err := sample.Data()
	if err != nil || view.Len() != len(pcm) || view.Format() != playdate.Sound16BitMono || view.SampleRate() != sampleRate {
		return errors.Join(err, g.close())
	}
	player, err := players.NewSamplePlayer(sample)
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.player = player
	controls, ok := player.(playdate.SamplePlayerControls)
	if !ok {
		return errors.Join(playdate.ErrAudioUnavailable, g.close())
	}
	g.controls = controls
	loopPlayer, ok := player.(playdate.LoopCallbackPlayer)
	if !ok {
		return errors.Join(playdate.ErrAudioUnavailable, g.close())
	}
	g.loopPlayer = loopPlayer
	if err = controls.SetPlayRange(0, sampleRate/4); err != nil {
		return errors.Join(err, g.close())
	}
	if err = loopPlayer.SetLoopCallback(func() { g.loops++ }); err != nil {
		return errors.Join(err, g.close())
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	input := context.Input()
	if input.Pressed.Has(playdate.ButtonA) {
		return true, g.player.PlayRepeated(0, 1)
	}
	if input.Pressed.Has(playdate.ButtonB) {
		return true, g.player.Stop()
	}
	context.Clear()
	context.DrawText("P9.2 samples", 12, 12)
	context.DrawText("A loop range  B stop", 12, 34)
	return input.Pressed != 0, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if g.player == nil {
		return nil
	}
	if event == playdate.LifecyclePause || event == playdate.LifecycleLock {
		return g.player.Pause()
	}
	if event == playdate.LifecycleResume || event == playdate.LifecycleUnlock {
		return g.player.Resume()
	}
	if event == playdate.LifecycleTerminate {
		return g.close()
	}
	return nil
}

func (g *game) close() error {
	var err error
	if g.player != nil {
		err = errors.Join(err, g.player.Close())
		g.player = nil
	}
	if g.sample != nil {
		err = errors.Join(err, g.sample.Close())
		g.sample = nil
	}
	return err
}
