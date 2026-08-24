// Package composition exercises the P6.1 transformed-bitmap and scoped-stencil API.
package composition

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

var errCapabilities = errors.New("P6.1 composition capabilities are unavailable")

type game struct {
	compositor playdate.BitmapCompositor
	offscreen  playdate.OffscreenGraphics
	primitives playdate.PrimitiveGraphics
	subject    playdate.Bitmap
	stencil    playdate.Bitmap
	canvas     playdate.Bitmap
	black      playdate.Paint
	white      playdate.Paint
}

// New creates the P6.1 bitmap-composition acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	compositor, compositorOK := context.(playdate.BitmapCompositor)
	offscreen, offscreenOK := context.(playdate.OffscreenGraphics)
	primitives, primitivesOK := context.(playdate.PrimitiveGraphics)
	if !compositorOK || !offscreenOK || !primitivesOK {
		return errCapabilities
	}
	black, err := playdate.SolidPaint(playdate.ColorBlack)
	if err != nil {
		return err
	}
	white, err := playdate.SolidPaint(playdate.ColorWhite)
	if err != nil {
		return err
	}
	subject, err := context.NewBitmap(64, 64)
	if err != nil {
		return err
	}
	stencil, err := context.NewBitmap(400, 240)
	if err != nil {
		subject.Close()
		return err
	}
	canvas, err := context.NewBitmap(400, 240)
	if err != nil {
		stencil.Close()
		subject.Close()
		return err
	}
	g.compositor, g.offscreen, g.primitives = compositor, offscreen, primitives
	g.subject, g.stencil, g.canvas, g.black, g.white = subject, stencil, canvas, black, white
	if err := g.prepare(); err != nil {
		g.closeBitmaps()
		return err
	}
	return nil
}

func (g *game) prepare() error {
	if err := g.subject.Fill(playdate.ColorWhite); err != nil {
		return err
	}
	if err := g.offscreen.DrawInto(g.subject, func() error {
		if err := g.primitives.FillRect(4, 4, 56, 56, g.black); err != nil {
			return err
		}
		return g.primitives.FillTriangle(8, 56, 32, 8, 56, 56, g.white)
	}); err != nil {
		return err
	}
	if err := g.stencil.Fill(playdate.ColorBlack); err != nil {
		return err
	}
	return g.offscreen.DrawInto(g.stencil, func() error {
		return g.primitives.FillEllipse(242, 84, 96, 96, 0, 360, g.white)
	})
}

func (g *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P6.1 rotate + scoped stencil", 12, 8)
	angle := context.Input().CrankAngle
	if err := g.compositor.DrawRotatedBitmap(g.subject, 110, 132, angle, .5, .5, 1.5, 1.5); err != nil {
		return false, err
	}
	if err := g.canvas.Clear(); err != nil {
		return false, err
	}
	if err := g.offscreen.DrawInto(g.canvas, func() error {
		return g.compositor.DrawRotatedBitmap(g.subject, 290, 132, angle, .5, .5, 1.5, 1.5)
	}); err != nil {
		return false, err
	}
	err := g.compositor.WithStencil(g.stencil, false, func() error {
		return context.DrawBitmap(g.canvas, 0, 0)
	})
	if err != nil {
		return false, err
	}
	context.DrawText("rotated", 82, 200)
	context.DrawText("stenciled", 256, 200)
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event != playdate.LifecycleTerminate {
		return nil
	}
	return g.closeBitmaps()
}

func (g *game) closeBitmaps() error {
	var first error
	if g.subject != nil {
		first = g.subject.Close()
		g.subject = nil
	}
	if g.stencil != nil {
		if err := g.stencil.Close(); first == nil {
			first = err
		}
		g.stencil = nil
	}
	if g.canvas != nil {
		if err := g.canvas.Close(); first == nil {
			first = err
		}
		g.canvas = nil
	}
	return first
}
