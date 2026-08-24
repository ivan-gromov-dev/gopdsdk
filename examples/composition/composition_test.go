package composition

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type bitmap struct{ closed bool }

func (*bitmap) Width() (int, error)       { return 64, nil }
func (*bitmap) Height() (int, error)      { return 64, nil }
func (*bitmap) Clear() error              { return nil }
func (*bitmap) Fill(playdate.Color) error { return nil }
func (b *bitmap) Close() error            { b.closed = true; return nil }

type context struct {
	bitmaps                  []*bitmap
	bitmapSizes              [][2]int
	angles                   []float32
	offscreen, primitives    int
	rotated, stencilSections int
	bitmapDraws              int
}

func (*context) Clear()                                               {}
func (*context) DrawText(string, int, int)                            {}
func (*context) CurrentTimeMilliseconds() uint32                      { return 0 }
func (*context) Input() playdate.Input                                { return playdate.Input{CrankAngle: 30} }
func (*context) LoadBitmap(string) (playdate.Bitmap, error)           { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (c *context) NewBitmap(width, height int) (playdate.Bitmap, error) {
	b := &bitmap{}
	c.bitmaps = append(c.bitmaps, b)
	c.bitmapSizes = append(c.bitmapSizes, [2]int{width, height})
	return b, nil
}
func (c *context) DrawBitmap(playdate.Bitmap, int, int) error {
	c.bitmapDraws++
	return nil
}
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*context) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) { return nil, nil }
func (*context) UpdateAndDrawSprites()                                              {}
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error)               { return nil, nil }
func (*context) LoadFilePlayer(string) (playdate.FilePlayer, error)                 { return nil, nil }
func (c *context) DrawInto(_ playdate.Bitmap, callback func() error) error {
	c.offscreen++
	return callback()
}
func (c *context) FillRect(int, int, int, int, playdate.Paint) error { c.primitives++; return nil }
func (c *context) FillTriangle(int, int, int, int, int, int, playdate.Paint) error {
	c.primitives++
	return nil
}
func (c *context) FillPolygon([]playdate.GraphicsPoint, playdate.PolygonFillRule, playdate.Paint) error {
	c.primitives++
	return nil
}
func (c *context) DrawRoundedRect(int, int, int, int, int, int, playdate.Paint) error {
	c.primitives++
	return nil
}
func (c *context) FillRoundedRect(int, int, int, int, int, playdate.Paint) error {
	c.primitives++
	return nil
}
func (c *context) FillEllipse(int, int, int, int, float32, float32, playdate.Paint) error {
	c.primitives++
	return nil
}
func (*context) DrawLine(int, int, int, int, int, playdate.Paint) error { return nil }
func (*context) DrawRect(int, int, int, int, playdate.Paint) error      { return nil }
func (*context) DrawEllipse(int, int, int, int, int, float32, float32, playdate.Paint) error {
	return nil
}
func (*context) DrawTriangle(int, int, int, int, int, int, int, playdate.Paint) error { return nil }
func (c *context) DrawRotatedBitmap(_ playdate.Bitmap, _, _ int, degrees, _, _, _, _ float32) error {
	c.rotated++
	c.angles = append(c.angles, degrees)
	return nil
}
func (c *context) WithStencil(_ playdate.Bitmap, _ bool, callback func() error) error {
	c.stencilSections++
	return callback()
}

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
	if context.offscreen != 3 || context.primitives != 3 || context.rotated != 2 || context.stencilSections != 1 || context.bitmapDraws != 1 {
		t.Fatalf("counts = offscreen %d primitives %d rotated %d stencils %d", context.offscreen, context.primitives, context.rotated, context.stencilSections)
	}
	if len(context.bitmapSizes) != 3 || context.bitmapSizes[0] != [2]int{64, 64} || context.bitmapSizes[1] != [2]int{400, 240} || context.bitmapSizes[2] != [2]int{400, 240} {
		t.Fatalf("bitmap sizes = %v", context.bitmapSizes)
	}
	if len(context.angles) != 2 || context.angles[0] != 30 || context.angles[1] != 30 {
		t.Fatalf("rotation angles = %v", context.angles)
	}
	if err := game.(playdate.LifecycleGame).HandleLifecycle(context, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if len(context.bitmaps) != 3 || !context.bitmaps[0].closed || !context.bitmaps[1].closed || !context.bitmaps[2].closed {
		t.Fatal("bitmaps were not closed")
	}
}
