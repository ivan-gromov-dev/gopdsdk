// Package framebuffer exercises the P3.2 framebuffer and offscreen-drawing API.
package framebuffer

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

var errCapabilities = errors.New("P3.2 graphics capabilities are unavailable")

type game struct {
	framebuffer playdate.FramebufferGraphics
	offscreen   playdate.OffscreenGraphics
	primitives  playdate.PrimitiveGraphics
	canvas      playdate.Bitmap
	black       playdate.Paint
}

// New creates the P3.2 framebuffer and offscreen acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	framebuffer, framebufferOK := context.(playdate.FramebufferGraphics)
	offscreen, offscreenOK := context.(playdate.OffscreenGraphics)
	primitives, primitivesOK := context.(playdate.PrimitiveGraphics)
	if !framebufferOK || !offscreenOK || !primitivesOK {
		return errCapabilities
	}
	black, err := playdate.SolidPaint(playdate.ColorBlack)
	if err != nil {
		return err
	}
	canvas, err := context.NewBitmap(128, 80)
	if err != nil {
		return err
	}
	if err := canvas.Fill(playdate.ColorWhite); err != nil {
		canvas.Close()
		return err
	}
	g.framebuffer, g.offscreen, g.primitives, g.canvas, g.black = framebuffer, offscreen, primitives, canvas, black
	if err := offscreen.DrawInto(canvas, func() error {
		if err := primitives.DrawRect(0, 0, 128, 80, black); err != nil {
			return err
		}
		if err := primitives.FillEllipse(18, 14, 44, 44, 0, 360, black); err != nil {
			return err
		}
		return primitives.FillTriangle(72, 60, 98, 14, 118, 60, black)
	}); err != nil {
		canvas.Close()
		g.canvas = nil
		return err
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P3.2 framebuffer + offscreen", 12, 8)
	if err := context.DrawBitmap(g.canvas, 20, 40); err != nil {
		return false, err
	}
	err := g.framebuffer.WithFramebuffer(func(frame playdate.Framebuffer) error {
		data, err := frame.Bytes()
		if err != nil {
			return err
		}
		for y := 144; y < 224; y++ {
			pattern := byte(0xaa)
			if y&1 != 0 {
				pattern = 0x55
			}
			row := y * frame.RowBytes()
			for column := 20; column < 49; column++ {
				data[row+column] = pattern
			}
		}
		return frame.MarkDirtyRows(144, 223)
	})
	if err != nil {
		return false, err
	}
	context.DrawText("owned bitmap", 32, 124)
	context.DrawText("zero-copy pixels", 224, 124)
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event != playdate.LifecycleTerminate || g.canvas == nil {
		return nil
	}
	err := g.canvas.Close()
	g.canvas = nil
	return err
}
