package fontsui

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type font struct{ closed int }

func (*font) TextWidth(text string) (int, error) { return len(text) * 8, nil }
func (*font) Height() (int, error)               { return 8, nil }
func (f *font) Close() error                     { f.closed++; return nil }

type context struct {
	input             playdate.Input
	font              *font
	drawn             []TextCommand
	tracking, leading int
	rectangles        int
}

func (*context) Clear()                    {}
func (*context) DrawText(string, int, int) {}
func (c *context) LoadFont(path string) (playdate.Font, error) {
	if path != "fonts/gopdsdk-ui" {
		return nil, errors.New("path")
	}
	c.font = &font{}
	return c.font, nil
}
func (c *context) DrawTextFont(_ playdate.Font, text string, x, y int) error {
	c.drawn = append(c.drawn, TextCommand{text, x, y})
	return nil
}
func (c *context) SetTextTracking(value int) { c.tracking = value }
func (c *context) TextTracking() int         { return c.tracking }
func (c *context) SetTextLeading(value int)  { c.leading = value }
func (c *context) DrawTextInRect(string, int, int, int, int, playdate.TextWrappingMode, playdate.TextAlignment) error {
	c.rectangles++
	return nil
}
func (*context) TextHeight(playdate.Font, string, int, playdate.TextWrappingMode, int, int) (int, error) {
	return 24, nil
}
func (*context) Glyph(playdate.Font, rune, rune) (playdate.FontGlyph, error) {
	return playdate.FontGlyph{Advance: 7, Kerning: -1}, nil
}
func (*context) LoadBitmap(string) (playdate.Bitmap, error)                         { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error)               { return nil, nil }
func (*context) NewBitmap(int, int) (playdate.Bitmap, error)                        { return nil, nil }
func (*context) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*context) CurrentTimeMilliseconds() uint32                                    { return 0 }
func (c *context) Input() playdate.Input                                            { return c.input }
func (*context) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) { return nil, nil }
func (*context) UpdateAndDrawSprites()                                              {}
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error)               { return nil, nil }
func (*context) LoadFilePlayer(string) (playdate.FilePlayer, error)                 { return nil, nil }

func TestLayoutPlanUsesFontMetrics(t *testing.T) {
	plan, err := LayoutPlan(State{Phase: GameOver, Score: 42}, &font{})
	if err != nil {
		t.Fatal(err)
	}
	if plan[1] != (TextCommand{"SCORE 42", 12, 22}) || plan[2] != (TextCommand{"GAME OVER", 164, 104}) || plan[3] != (TextCommand{"A:RESTART", 164, 120}) {
		t.Fatalf("plan = %#v", plan)
	}
	if plan[0].Text != "P7.3 TEXT + FONTS" {
		t.Fatalf("acceptance title = %q", plan[0].Text)
	}
}

func TestHUDPauseGameOverAndRestartFlow(t *testing.T) {
	c := &context{}
	g := New().(*game)
	if err := g.Init(c); err != nil {
		t.Fatal(err)
	}
	if c.tracking != 1 || c.leading != 2 || g.textHeight != 24 || g.glyphAdvance != 7 {
		t.Fatalf("text metrics: tracking=%d leading=%d height=%d advance=%d", c.tracking, c.leading, g.textHeight, g.glyphAdvance)
	}
	c.input.Pressed = playdate.ButtonA
	if _, err := g.Update(c); err != nil || g.state.Score != 1 {
		t.Fatalf("score: %+v %v", g.state, err)
	}
	if c.rectangles != 1 {
		t.Fatalf("bounded text draws = %d", c.rectangles)
	}
	if err := g.HandleLifecycle(c, playdate.LifecyclePause); err != nil || g.state.Phase != Paused {
		t.Fatalf("pause: %+v %v", g.state, err)
	}
	if _, err := g.Update(c); err != nil || c.drawn[len(c.drawn)-2].Text != "PAUSED" {
		t.Fatalf("pause draw: %#v %v", c.drawn, err)
	}
	if err := g.HandleLifecycle(c, playdate.LifecycleResume); err != nil {
		t.Fatal(err)
	}
	c.input.Pressed = playdate.ButtonB
	if _, err := g.Update(c); err != nil || g.state.Phase != GameOver {
		t.Fatalf("game over: %+v %v", g.state, err)
	}
	c.input.Pressed = playdate.ButtonA
	if _, err := g.Update(c); err != nil || g.state != (State{Phase: Playing}) {
		t.Fatalf("restart: %+v %v", g.state, err)
	}
	if err := g.HandleLifecycle(c, playdate.LifecycleTerminate); err != nil || c.font.closed != 1 {
		t.Fatalf("close=%d err=%v", c.font.closed, err)
	}
}
