// Package sprites exercises the display, display-introspection, and
// sprite-redraw API.
package sprites

import (
	"errors"
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	spriteCount       = 3
	pulseFrames       = 30
	presentationModes = 6
)

var errCapabilities = errors.New("display and sprite-redraw capabilities are unavailable")

type game struct {
	bitmaps      [2]playdate.Bitmap
	sprites      [spriteCount]playdate.Sprite
	x            [spriteCount]float32
	frame        int
	fullRedraw   bool
	presentation int
	closed       bool
}

// New creates the sprite display-list acceptance game.
func New() playdate.Game { return &game{x: [spriteCount]float32{80, 200, 320}} }

func (g *game) Init(context playdate.Context) error {
	display, displayOK := context.(playdate.Display)
	redraw, redrawOK := context.(playdate.SpriteRedraw)
	_, offscreenOK := context.(playdate.OffscreenGraphics)
	_, fillOK := context.(interface {
		FillRect(x, y, width, height int, paint playdate.Paint) error
	})
	if !displayOK || !redrawOK || !offscreenOK || !fillOK {
		return errCapabilities
	}
	if err := display.SetRefreshRate(30); err != nil {
		return err
	}
	redraw.SetAlwaysRedraw(false)
	bitmap, err := context.NewBitmap(24, 24)
	if err != nil {
		return err
	}
	g.bitmaps[0] = bitmap
	if err := bitmap.Fill(playdate.ColorBlack); err != nil {
		return errors.Join(err, g.close())
	}
	for index := range g.sprites {
		sprite, createErr := context.NewSprite()
		if createErr != nil {
			return errors.Join(createErr, g.close())
		}
		g.sprites[index] = sprite
		spriteBitmap := bitmap
		if index == 2 {
			pulse, pulseErr := context.NewBitmap(80, 32)
			if pulseErr != nil {
				return errors.Join(pulseErr, g.close())
			}
			g.bitmaps[1] = pulse
			if fillErr := pulse.Fill(playdate.ColorBlack); fillErr != nil {
				return errors.Join(fillErr, g.close())
			}
			spriteBitmap = pulse
		}
		if setupErr := setupSprite(sprite, spriteBitmap, g.x[index], float32(112+index*48), index); setupErr != nil {
			return errors.Join(setupErr, g.close())
		}
	}
	g.drawInstructions(context)
	return nil
}

func setupSprite(sprite playdate.Sprite, bitmap playdate.Bitmap, x, y float32, z int) error {
	if err := sprite.SetBitmap(bitmap); err != nil {
		return err
	}
	if err := sprite.SetPosition(x, y); err != nil {
		return err
	}
	if err := sprite.SetVisible(true); err != nil {
		return err
	}
	if err := sprite.SetZIndex(z); err != nil {
		return err
	}
	return sprite.Add()
}

func (g *game) Update(context playdate.Context) (bool, error) {
	g.frame++
	input := context.Input()
	if input.Pressed.Has(playdate.ButtonA) {
		g.fullRedraw = !g.fullRedraw
		context.(playdate.SpriteRedraw).SetAlwaysRedraw(g.fullRedraw)
		g.drawInstructions(context)
	}
	if input.Pressed.Has(playdate.ButtonB) {
		g.presentation = (g.presentation + 1) % presentationModes
		if err := g.applyPresentation(context.(playdate.Display)); err != nil {
			return false, err
		}
		g.drawInstructions(context)
	}
	if input.Pressed.Has(playdate.ButtonUp) {
		g.fullRedraw = false
		g.presentation = 0
		context.(playdate.SpriteRedraw).SetAlwaysRedraw(false)
		if err := g.applyPresentation(context.(playdate.Display)); err != nil {
			return false, err
		}
		g.drawInstructions(context)
	}
	g.x[0] += input.CrankDelta
	if g.x[0] < 12 {
		g.x[0] = 388
	}
	if g.x[0] > 388 {
		g.x[0] = 12
	}
	if err := g.sprites[0].SetPosition(g.x[0], 112); err != nil {
		return false, err
	}
	if err := g.sprites[1].MoveBy(1, 0); err != nil {
		return false, err
	}
	g.x[1]++
	if g.x[1] > 388 {
		g.x[1] = 12
		if err := g.sprites[1].SetPosition(g.x[1], 160); err != nil {
			return false, err
		}
	}
	if g.frame%pulseFrames == 0 {
		pulse := g.frame / pulseFrames
		color := playdate.ColorBlack
		if ((pulse-1)/2)%2 == 0 {
			color = playdate.ColorWhite
		}
		x := float32(0)
		if pulse%2 == 0 {
			x = 40
		}
		paint, err := playdate.SolidPaint(color)
		if err != nil {
			return false, err
		}
		xi := int(x)
		if err := context.(playdate.OffscreenGraphics).DrawInto(g.bitmaps[1], func() error {
			return context.(interface {
				FillRect(x, y, width, height int, paint playdate.Paint) error
			}).FillRect(xi, 0, 40, 32, paint)
		}); err != nil {
			return false, err
		}
		if err := g.sprites[2].MarkDirtyRect(playdate.Rect{X: x, Width: 40, Height: 32}); err != nil {
			return false, err
		}
		if err := context.(playdate.SpriteRedraw).AddDirtyRect(8, 96, 384, 2); err != nil {
			return false, err
		}
	}
	context.UpdateAndDrawSprites()
	g.drawOverlay(context)
	return true, nil
}

func (g *game) drawInstructions(context playdate.Context) {
	context.Clear()
	g.drawOverlay(context)
}

func (g *game) drawOverlay(context playdate.Context) {
	mode := "DIRTY"
	if g.fullRedraw {
		mode = "FULL"
	}
	display := context.(playdate.Display)
	context.DrawText("P7.4 "+mode+" / "+g.presentationName(), 6, 4)
	context.DrawText("A redraw  B display  UP reset", 6, 24)
	context.DrawText("Crank: top square", 6, 44)
	context.DrawText("Bar halves change separately", 6, 64)
	metrics := strconv.Itoa(display.Width()) + "x" + strconv.Itoa(display.Height()) +
		"  target " + strconv.FormatFloat(float64(display.RefreshRate()), 'f', 1, 32) +
		"  actual " + strconv.FormatFloat(float64(display.FPS()), 'f', 1, 32) + " FPS"
	context.DrawText(metrics, 6, 84)
}

func (g *game) presentationName() string {
	return [...]string{"NORMAL", "INVERT", "MOSAIC", "FLIP-X", "OFFSET", "SCALE-2 (200x120)"}[g.presentation]
}

func (g *game) applyPresentation(display playdate.Display) error {
	display.SetInverted(false)
	display.SetFlipped(false, false)
	display.SetOffset(0, 0)
	if err := display.SetScale(1); err != nil {
		return err
	}
	if err := display.SetMosaic(0, 0); err != nil {
		return err
	}
	switch g.presentation {
	case 1:
		display.SetInverted(true)
	case 2:
		return display.SetMosaic(2, 2)
	case 3:
		display.SetFlipped(true, false)
	case 4:
		display.SetOffset(16, 12)
	case 5:
		return display.SetScale(2)
	}
	return nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
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
	var err error
	for _, sprite := range g.sprites {
		if sprite != nil {
			err = errors.Join(err, sprite.Close())
		}
	}
	for _, bitmap := range g.bitmaps {
		if bitmap != nil {
			err = errors.Join(err, bitmap.Close())
		}
	}
	return err
}
