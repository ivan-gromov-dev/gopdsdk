package framebuffer

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type bitmap struct {
	filled playdate.Color
	closed bool
}

func (*bitmap) Width() (int, error)               { return 128, nil }
func (*bitmap) Height() (int, error)              { return 80, nil }
func (b *bitmap) Clear() error                    { return b.Fill(playdate.ColorClear) }
func (b *bitmap) Fill(color playdate.Color) error { b.filled = color; return nil }
func (b *bitmap) Close() error                    { b.closed = true; return nil }

type frame struct {
	data                 []byte
	dirtyStart, dirtyEnd int
}

func (*frame) Width() int                              { return 400 }
func (*frame) Height() int                             { return 240 }
func (*frame) RowBytes() int                           { return 52 }
func (f *frame) Bytes() ([]byte, error)                { return f.data, nil }
func (*frame) Pixel(int, int) (playdate.Color, error)  { return playdate.ColorBlack, nil }
func (*frame) SetPixel(int, int, playdate.Color) error { return nil }
func (f *frame) MarkDirtyRows(start, end int) error {
	f.dirtyStart, f.dirtyEnd = start, end
	return nil
}

type context struct {
	bitmap           *bitmap
	frame            *frame
	offscreen, draws int
}

func (*context) Clear()                                               {}
func (*context) DrawText(string, int, int)                            {}
func (*context) CurrentTimeMilliseconds() uint32                      { return 0 }
func (*context) Input() playdate.Input                                { return playdate.Input{} }
func (*context) LoadBitmap(string) (playdate.Bitmap, error)           { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (c *context) NewBitmap(int, int) (playdate.Bitmap, error) {
	c.bitmap = &bitmap{}
	return c.bitmap, nil
}
func (*context) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*context) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) { return nil, nil }
func (*context) UpdateAndDrawSprites()                                              {}
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error)               { return nil, nil }
func (*context) LoadFilePlayer(string) (playdate.FilePlayer, error)                 { return nil, nil }
func (c *context) WithFramebuffer(callback func(playdate.Framebuffer) error) error {
	c.frame = &frame{data: make([]byte, 52*240)}
	return callback(c.frame)
}
func (c *context) DrawInto(_ playdate.Bitmap, callback func() error) error {
	c.offscreen++
	return callback()
}
func (c *context) draw() error                                            { c.draws++; return nil }
func (c *context) DrawLine(int, int, int, int, int, playdate.Paint) error { return c.draw() }
func (c *context) DrawRect(int, int, int, int, playdate.Paint) error      { return c.draw() }
func (c *context) FillRect(int, int, int, int, playdate.Paint) error      { return c.draw() }
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

func TestAcceptanceScene(t *testing.T) {
	ctx := &context{}
	value := New()
	g := value.(*game)
	if err := g.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.offscreen != 1 || ctx.draws != 3 || ctx.bitmap.filled != playdate.ColorWhite {
		t.Fatalf("init offscreen=%d draws=%d fill=%d", ctx.offscreen, ctx.draws, ctx.bitmap.filled)
	}
	refresh, err := g.Update(ctx)
	if err != nil || !refresh {
		t.Fatalf("Update()=%v,%v", refresh, err)
	}
	if ctx.frame.dirtyStart != 144 || ctx.frame.dirtyEnd != 223 {
		t.Fatalf("dirty=%d..%d", ctx.frame.dirtyStart, ctx.frame.dirtyEnd)
	}
	if got := ctx.frame.data[144*52+20]; got != 0xaa {
		t.Fatalf("even pattern=%02x", got)
	}
	if got := ctx.frame.data[145*52+20]; got != 0x55 {
		t.Fatalf("odd pattern=%02x", got)
	}
	if err := g.HandleLifecycle(ctx, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if !ctx.bitmap.closed {
		t.Fatal("bitmap was not closed")
	}
}
