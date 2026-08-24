package navigation

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type context struct {
	input  playdate.Input
	exited bool
	texts  []string
}

func (c *context) Clear()                                             { c.texts = nil }
func (c *context) DrawText(text string, _, _ int)                     { c.texts = append(c.texts, text) }
func (*context) CurrentTimeMilliseconds() uint32                      { return 0 }
func (c *context) Input() playdate.Input                              { return c.input }
func (c *context) ExitToLauncher()                                    { c.exited = true }
func (*context) LoadBitmap(string) (playdate.Bitmap, error)           { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (*context) NewBitmap(int, int) (playdate.Bitmap, error)          { return nil, nil }
func (*context) DrawBitmap(playdate.Bitmap, int, int) error           { return nil }
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error {
	return nil
}
func (*context) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) { return nil, nil }
func (*context) UpdateAndDrawSprites()                                              {}
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error)               { return nil, nil }
func (*context) LoadFilePlayer(string) (playdate.FilePlayer, error)                 { return nil, nil }

func TestPlayReturnsToMenuAndExitUsesLauncher(t *testing.T) {
	g := New().(*game)
	c := &context{}
	if err := g.Init(c); err != nil {
		t.Fatal(err)
	}

	c.input.Pressed = playdate.ButtonA
	if _, err := g.Update(c); err != nil || g.phase != phasePlaying {
		t.Fatalf("Play transition: phase=%v error=%v", g.phase, err)
	}
	c.input.Pressed = playdate.ButtonB
	if _, err := g.Update(c); err != nil || g.phase != phaseMenu {
		t.Fatalf("menu return: phase=%v error=%v", g.phase, err)
	}
	c.input.Pressed = playdate.ButtonDown
	if _, err := g.Update(c); err != nil || g.selected != 1 {
		t.Fatalf("Exit selection: selected=%d error=%v", g.selected, err)
	}
	c.input.Pressed = playdate.ButtonA
	if _, err := g.Update(c); err != nil || !c.exited {
		t.Fatalf("Exit: called=%t error=%v", c.exited, err)
	}
}
