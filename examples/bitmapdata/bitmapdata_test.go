package bitmapdata

import (
	"reflect"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testBitmap struct {
	name string
	ops  *[]string
}

func (*testBitmap) Width() (int, error)         { return 64, nil }
func (*testBitmap) Height() (int, error)        { return 64, nil }
func (b *testBitmap) Clear() error              { return b.Fill(playdate.ColorClear) }
func (b *testBitmap) Fill(playdate.Color) error { *b.ops = append(*b.ops, "fill:"+b.name); return nil }
func (b *testBitmap) Close() error              { *b.ops = append(*b.ops, "close:"+b.name); return nil }

type testData struct{ dirty bool }

func (*testData) Width() int                                { return 64 }
func (*testData) Height() int                               { return 64 }
func (*testData) RowBytes() int                             { return 8 }
func (*testData) Bytes() ([]byte, error)                    { return make([]byte, 512), nil }
func (*testData) MaskBytes() ([]byte, error)                { return make([]byte, 512), nil }
func (d *testData) Dirty() (bool, error)                    { return d.dirty, nil }
func (d *testData) MarkDirty() error                        { d.dirty = true; return nil }
func (*testData) Pixel(int, int) (playdate.Color, error)    { return playdate.ColorWhite, nil }
func (d *testData) SetPixel(int, int, playdate.Color) error { d.dirty = true; return nil }

type testTable struct {
	frame playdate.Bitmap
	ops   *[]string
}

func (t *testTable) Frame(int) (playdate.Bitmap, error) { return t.frame, nil }
func (t *testTable) Close() error                       { *t.ops = append(*t.ops, "close:table"); return nil }

type testContext struct {
	ops      []string
	sequence int
	mask     playdate.Bitmap
	frame    playdate.Bitmap
}

func (c *testContext) bitmap(name string) playdate.Bitmap {
	return &testBitmap{name: name, ops: &c.ops}
}
func (*testContext) Clear()                                                 {}
func (*testContext) DrawText(string, int, int)                              {}
func (*testContext) CurrentTimeMilliseconds() uint32                        { return 0 }
func (*testContext) Input() playdate.Input                                  { return playdate.Input{} }
func (*testContext) NewSprite() (playdate.Sprite, error)                    { return nil, nil }
func (*testContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite { return nil }
func (*testContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite     { return nil }
func (*testContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (*testContext) UpdateAndDrawSprites()                                {}
func (*testContext) LoadSoundEffect(string) (playdate.SoundEffect, error) { return nil, nil }
func (*testContext) LoadFilePlayer(string) (playdate.FilePlayer, error)   { return nil, nil }
func (c *testContext) LoadBitmap(string) (playdate.Bitmap, error) {
	c.ops = append(c.ops, "load")
	return c.bitmap("source"), nil
}
func (*testContext) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (c *testContext) NewBitmap(int, int) (playdate.Bitmap, error) {
	c.sequence++
	return c.bitmap([]string{"loaded", "mask"}[c.sequence-1]), nil
}
func (c *testContext) DrawBitmap(playdate.Bitmap, int, int) error {
	c.ops = append(c.ops, "draw")
	return nil
}
func (c *testContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error {
	c.ops = append(c.ops, "draw-scaled")
	return nil
}
func (c *testContext) WithBitmapData(_ playdate.Bitmap, callback func(playdate.BitmapData) error) error {
	c.ops = append(c.ops, "data")
	return callback(&testData{})
}
func (c *testContext) CopyBitmap(playdate.Bitmap) (playdate.Bitmap, error) {
	c.ops = append(c.ops, "copy")
	return c.bitmap("copy"), nil
}
func (c *testContext) LoadIntoBitmap(string, playdate.Bitmap) error {
	c.ops = append(c.ops, "load-into")
	return nil
}
func (c *testContext) NewBitmapTable(int, int, int) (playdate.BitmapTable, error) {
	c.ops = append(c.ops, "new-table")
	c.frame = c.bitmap("frame")
	return &testTable{frame: c.frame, ops: &c.ops}, nil
}
func (c *testContext) LoadIntoBitmapTable(string, playdate.BitmapTable) error {
	c.ops = append(c.ops, "load-table")
	return nil
}
func (c *testContext) SetBitmapMask(_ playdate.Bitmap, mask playdate.Bitmap) error {
	c.ops = append(c.ops, "set-mask")
	c.mask = mask
	return nil
}
func (c *testContext) ClearBitmapMask(playdate.Bitmap) error {
	c.ops = append(c.ops, "clear-mask")
	return nil
}
func (c *testContext) BitmapMask(playdate.Bitmap) (playdate.Bitmap, bool, error) {
	c.ops = append(c.ops, "get-mask")
	return c.bitmap("mask-view"), true, nil
}
func (c *testContext) CheckBitmapMaskCollision(playdate.Bitmap, int, int, playdate.BitmapFlip, playdate.Bitmap, int, int, playdate.BitmapFlip, int, int, int, int) (bool, error) {
	c.ops = append(c.ops, "collision")
	return true, nil
}
func (c *testContext) RotatedBitmap(playdate.Bitmap, float32, float32, float32) (playdate.Bitmap, int, error) {
	c.ops = append(c.ops, "rotate")
	return c.bitmap("rotated"), 128, nil
}
func (c *testContext) CopyDisplayBuffer() (playdate.Bitmap, error) {
	c.ops = append(c.ops, "display-copy")
	return c.bitmap("snapshot"), nil
}

func TestCompleteP72AcceptanceLifecycle(t *testing.T) {
	context := &testContext{}
	game := New().(*game)
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	if refresh, err := game.Update(context); err != nil || !refresh {
		t.Fatalf("Update = %v, %v", refresh, err)
	}
	if err := game.HandleLifecycle(context, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	wantPrefix := []string{"load", "copy", "load-into", "fill:mask", "data", "set-mask", "get-mask", "data", "close:mask-view", "collision", "rotate", "display-copy", "new-table", "load-table"}
	if !reflect.DeepEqual(context.ops[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("operations = %v", context.ops)
	}
	if !game.dirty || !game.collides {
		t.Fatalf("assertions = dirty %v collision %v", game.dirty, game.collides)
	}
}
