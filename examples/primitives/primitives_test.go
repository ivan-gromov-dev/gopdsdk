package primitives

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type context struct{ draws, stateChanges int }

func (*context) Clear()                                                             {}
func (*context) DrawText(string, int, int)                                          {}
func (*context) CurrentTimeMilliseconds() uint32                                    { return 0 }
func (*context) Input() playdate.Input                                              { return playdate.Input{} }
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
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error)               { return nil, nil }
func (*context) LoadFilePlayer(string) (playdate.FilePlayer, error)                 { return nil, nil }
func (c *context) draw() error                                                      { c.draws++; return nil }
func (c *context) DrawLine(int, int, int, int, int, playdate.Paint) error           { return c.draw() }
func (c *context) DrawRect(int, int, int, int, playdate.Paint) error                { return c.draw() }
func (c *context) FillRect(int, int, int, int, playdate.Paint) error                { return c.draw() }
func (c *context) DrawEllipse(int, int, int, int, int, float32, float32, playdate.Paint) error {
	return c.draw()
}
func (c *context) FillEllipse(int, int, int, int, float32, float32, playdate.Paint) error {
	return c.draw()
}
func (c *context) DrawTriangle(int, int, int, int, int, int, int, playdate.Paint) error {
	return c.draw()
}
func (c *context) FillTriangle(int, int, int, int, int, int, playdate.Paint) error { return c.draw() }
func (c *context) FillPolygon([]playdate.GraphicsPoint, playdate.PolygonFillRule, playdate.Paint) error {
	return c.draw()
}
func (c *context) DrawRoundedRect(int, int, int, int, int, int, playdate.Paint) error {
	return c.draw()
}
func (c *context) FillRoundedRect(int, int, int, int, int, playdate.Paint) error { return c.draw() }
func (c *context) SetClipRect(int, int, int, int) error                          { c.stateChanges++; return nil }
func (c *context) ClearClipRect()                                                { c.stateChanges++ }
func (c *context) SetDrawOffset(int, int)                                        { c.stateChanges++ }
func (c *context) SetDrawMode(playdate.DrawMode) error                           { c.stateChanges++; return nil }
func (c *context) SetLineCapStyle(playdate.LineCapStyle) error                   { c.stateChanges++; return nil }
func (c *context) SetBackgroundColor(playdate.Color) error                       { c.stateChanges++; return nil }
func (c *context) SetScreenClipRect(int, int, int, int) error                    { c.stateChanges++; return nil }

func TestAcceptanceScene(t *testing.T) {
	context := &context{}
	game := New()
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	refresh, err := game.Update(context)
	if err != nil || !refresh {
		t.Fatalf("Update() = %v, %v", refresh, err)
	}
	if context.draws != 15 {
		t.Fatalf("draws = %d, want 15", context.draws)
	}
	if context.stateChanges != 9 {
		t.Fatalf("state changes = %d, want 9", context.stateChanges)
	}
}
