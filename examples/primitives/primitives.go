// Package primitives exercises the P7.1 drawing-primitives and graphics-state API.
package primitives

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

var errCapabilities = errors.New("P7.1 graphics capabilities are unavailable")

type game struct {
	primitives playdate.PrimitiveGraphics
	state      playdate.GraphicsState
	black      playdate.Paint
	white      playdate.Paint
	pattern    playdate.Paint
}

// New creates the P7.1 primitive-rendering acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	primitives, primitivesOK := context.(playdate.PrimitiveGraphics)
	state, stateOK := context.(playdate.GraphicsState)
	if !primitivesOK || !stateOK {
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
	g.primitives, g.state, g.black, g.white = primitives, state, black, white
	g.pattern = playdate.PatternPaint(
		[8]byte{0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55},
		[8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	)
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P7.1 primitives + state", 12, 8)
	if err := g.state.SetBackgroundColor(playdate.ColorWhite); err != nil {
		return false, err
	}
	if err := g.state.SetLineCapStyle(playdate.LineCapRound); err != nil {
		return false, err
	}
	if err := g.state.SetScreenClipRect(0, 0, 400, 240); err != nil {
		return false, err
	}

	if err := g.primitives.DrawLine(16, 42, 112, 42, 3, g.black); err != nil {
		return false, err
	}
	if err := g.primitives.DrawRect(16, 56, 96, 44, g.black); err != nil {
		return false, err
	}
	if err := g.primitives.FillRect(122, 56, 52, 44, g.pattern); err != nil {
		return false, err
	}
	if err := g.primitives.DrawEllipse(188, 48, 72, 56, 3, 0, 360, g.black); err != nil {
		return false, err
	}
	if err := g.primitives.FillEllipse(274, 48, 72, 56, 0, 360, g.pattern); err != nil {
		return false, err
	}
	if err := g.primitives.DrawTriangle(24, 160, 76, 112, 124, 160, 3, g.black); err != nil {
		return false, err
	}
	if err := g.primitives.FillTriangle(146, 160, 198, 112, 246, 160, g.pattern); err != nil {
		return false, err
	}
	if err := g.primitives.FillPolygon([]playdate.GraphicsPoint{{X: 274, Y: 112}, {X: 346, Y: 112}, {X: 334, Y: 160}, {X: 286, Y: 160}}, playdate.PolygonFillEvenOdd, g.pattern); err != nil {
		return false, err
	}
	if err := g.primitives.DrawRoundedRect(270, 176, 54, 42, 8, 3, g.black); err != nil {
		return false, err
	}
	if err := g.primitives.FillRoundedRect(332, 176, 54, 42, 8, g.pattern); err != nil {
		return false, err
	}

	if err := g.state.SetClipRect(274, 112, 72, 48); err != nil {
		return false, err
	}
	g.state.SetDrawOffset(12, 8)
	if err := g.primitives.FillRect(262, 104, 96, 64, g.black); err != nil {
		return false, err
	}
	g.state.SetDrawOffset(0, 0)
	g.state.ClearClipRect()

	if err := g.state.SetDrawMode(playdate.DrawModeXOR); err != nil {
		return false, err
	}
	if err := g.primitives.FillEllipse(32, 182, 52, 36, 0, 360, playdate.XORPaint()); err != nil {
		return false, err
	}
	if err := g.primitives.FillEllipse(58, 182, 52, 36, 0, 360, playdate.XORPaint()); err != nil {
		return false, err
	}
	if err := g.state.SetDrawMode(playdate.DrawModeCopy); err != nil {
		return false, err
	}
	if err := g.primitives.FillRect(146, 184, 100, 34, g.black); err != nil {
		return false, err
	}
	context.DrawText("CLIP", 286, 130)
	if err := g.primitives.FillRect(158, 194, 76, 14, g.white); err != nil {
		return false, err
	}
	return true, nil
}
