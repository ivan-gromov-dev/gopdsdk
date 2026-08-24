// Package video exercises the P6.3 owned PDV player API.
package video

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const videoPath = "sample.pdv"
const audioPath = "audio/sample"

var errCapabilities = errors.New("P6.3 video capabilities are unavailable")

type game struct {
	player  playdate.VideoPlayer
	audio   playdate.SamplePlayer
	canvas  playdate.Bitmap
	info    playdate.VideoInfo
	frame   int
	elapsed float32
	paused  bool
	screen  bool
	closed  bool
}

// New creates the P6.3 video acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	videos, ok := context.(playdate.Videos)
	samples, samplesOK := context.(playdate.SamplePlayers)
	if !ok || !samplesOK {
		return errCapabilities
	}
	player, err := videos.LoadVideo(videoPath)
	if err != nil {
		return err
	}
	g.player = player
	audio, err := samples.LoadSamplePlayer(audioPath)
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.audio = audio
	if g.info, err = player.Info(); err != nil {
		return errors.Join(err, g.close())
	}
	canvas, err := context.NewBitmap(g.info.Width, g.info.Height)
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.canvas = canvas
	if err = player.SetContext(canvas); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.render(); err != nil {
		return errors.Join(err, g.close())
	}
	if err = g.audio.Play(); err != nil {
		return errors.Join(err, g.close())
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	if g.closed || g.player == nil {
		return false, playdate.ErrVideoClosed
	}
	input := context.Input()
	if input.Pressed.Has(playdate.ButtonA) {
		g.paused = !g.paused
		var err error
		if g.paused {
			err = g.audio.Pause()
		} else {
			err = g.audio.Resume()
		}
		if err != nil {
			return false, err
		}
	}
	if input.Pressed.Has(playdate.ButtonB) {
		g.screen = !g.screen
		var err error
		if g.screen {
			err = g.player.UseScreenContext()
		} else {
			err = g.player.SetContext(g.canvas)
		}
		if err != nil {
			return false, err
		}
	}

	next := g.frame
	if input.Pressed.Has(playdate.ButtonLeft) {
		next--
		g.elapsed = 0
	}
	if input.Pressed.Has(playdate.ButtonRight) {
		next++
		g.elapsed = 0
	}
	if !g.paused && g.info.FrameRate > 0 {
		g.elapsed += input.DeltaSeconds
		frameDuration := float32(1) / g.info.FrameRate
		for g.elapsed >= frameDuration {
			g.elapsed -= frameDuration
			next++
		}
	}
	wrapped := false
	if g.info.FrameCount > 0 {
		wrapped = next >= g.info.FrameCount
		next = (next%g.info.FrameCount + g.info.FrameCount) % g.info.FrameCount
	}
	if wrapped && !g.paused {
		if err := g.audio.Stop(); err != nil {
			return false, err
		}
		if err := g.audio.Play(); err != nil {
			return false, err
		}
	}
	if next != g.frame || input.Pressed.Has(playdate.ButtonB) {
		g.frame = next
		if err := g.render(); err != nil {
			return false, err
		}
	}
	if !g.screen {
		context.Clear()
		x := (400 - g.info.Width) / 2
		y := (240 - g.info.Height) / 2
		if err := context.DrawBitmap(g.canvas, x, y); err != nil {
			return false, err
		}
		state := "playing"
		if g.paused {
			state = "paused"
		}
		context.DrawText("P6.3 PDV "+state+"  A pause  B target", 8, 8)
	}
	return true, nil
}

func (g *game) render() error { return g.player.RenderFrame(g.frame) }

func (g *game) close() error {
	if g.closed {
		return nil
	}
	g.closed = true
	var err error
	if g.player != nil {
		err = errors.Join(err, g.player.Close())
		g.player = nil
	}
	if g.audio != nil {
		err = errors.Join(err, g.audio.Close())
		g.audio = nil
	}
	if g.canvas != nil {
		err = errors.Join(err, g.canvas.Close())
		g.canvas = nil
	}
	return err
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecycleTerminate {
		return g.close()
	}
	return nil
}
