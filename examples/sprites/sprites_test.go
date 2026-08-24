package sprites

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testBitmap struct{ operations *[]string }

func (*testBitmap) Width() (int, error)  { return 24, nil }
func (*testBitmap) Height() (int, error) { return 24, nil }
func (b *testBitmap) Clear() error       { return b.Fill(playdate.ColorClear) }
func (b *testBitmap) Fill(playdate.Color) error {
	*b.operations = append(*b.operations, "bitmap.fill")
	return nil
}
func (b *testBitmap) Close() error { *b.operations = append(*b.operations, "bitmap.close"); return nil }

type testSprite struct {
	name       string
	operations *[]string
}

func (s *testSprite) record(value string) error {
	*s.operations = append(*s.operations, s.name+"."+value)
	return nil
}
func (s *testSprite) SetBitmap(playdate.Bitmap) error              { return s.record("bitmap") }
func (s *testSprite) SetCenter(float32, float32) error             { return s.record("center") }
func (*testSprite) Center() (float32, float32, error)              { return 0, 0, nil }
func (s *testSprite) SetBounds(playdate.Rect) error                { return s.record("bounds") }
func (*testSprite) Bounds() (playdate.Rect, error)                 { return playdate.Rect{}, nil }
func (s *testSprite) SetPosition(float32, float32) error           { return s.record("position") }
func (*testSprite) Position() (float32, float32, error)            { return 0, 0, nil }
func (s *testSprite) MoveBy(float32, float32) error                { return s.record("move") }
func (s *testSprite) SetVisible(bool) error                        { return s.record("visible") }
func (*testSprite) Visible() (bool, error)                         { return true, nil }
func (s *testSprite) SetZIndex(int) error                          { return s.record("z") }
func (*testSprite) ZIndex() (int, error)                           { return 0, nil }
func (s *testSprite) SetImageFlip(playdate.BitmapFlip) error       { return s.record("flip") }
func (*testSprite) ImageFlip() (playdate.BitmapFlip, error)        { return playdate.BitmapUnflipped, nil }
func (s *testSprite) SetDrawMode(playdate.DrawMode) error          { return s.record("drawMode") }
func (s *testSprite) SetOpaque(bool) error                         { return s.record("opaque") }
func (s *testSprite) SetStencilImage(playdate.Bitmap, bool) error  { return s.record("stencilImage") }
func (s *testSprite) SetStencilPattern([8]byte) error              { return s.record("stencilPattern") }
func (s *testSprite) ClearStencil() error                          { return s.record("clearStencil") }
func (s *testSprite) SetClipRect(int, int, int, int) error         { return s.record("clip") }
func (s *testSprite) ClearClipRect() error                         { return s.record("clearClip") }
func (s *testSprite) SetIgnoresDrawOffset(bool) error              { return s.record("drawOffset") }
func (s *testSprite) SetTileMap(playdate.SpriteTileMap) error      { return s.record("tilemap") }
func (s *testSprite) ClearTileMap() error                          { return s.record("clearTilemap") }
func (*testSprite) TileMap() (playdate.SpriteTileMap, bool, error) { return nil, false, nil }
func (s *testSprite) SetUpdatesEnabled(bool) error                 { return s.record("updates") }
func (*testSprite) UpdatesEnabled() (bool, error)                  { return true, nil }
func (s *testSprite) SetCollisionsEnabled(bool) error              { return s.record("collisions") }
func (*testSprite) CollisionsEnabled() (bool, error)               { return true, nil }
func (s *testSprite) SetCollideRect(playdate.Rect) error           { return s.record("collideRect") }
func (*testSprite) CollideRect() (playdate.Rect, error)            { return playdate.Rect{}, nil }
func (s *testSprite) ClearCollideRect() error                      { return s.record("clearCollideRect") }
func (s *testSprite) SetTag(uint8) error                           { return s.record("tag") }
func (*testSprite) Tag() (uint8, error)                            { return 0, nil }
func (s *testSprite) MarkDirty() error                             { return s.record("dirty") }
func (s *testSprite) MarkDirtyRect(playdate.Rect) error            { return s.record("dirtyRect") }
func (s *testSprite) MoveWithCollisions(float32, float32) (playdate.MoveResult, error) {
	return playdate.MoveResult{}, s.record("collideMove")
}
func (s *testSprite) CheckCollisions(float32, float32) (playdate.MoveResult, error) {
	return playdate.MoveResult{}, nil
}
func (s *testSprite) SetDrawCallback(playdate.SpriteDrawCallback) error {
	return s.record("drawCallback")
}
func (s *testSprite) SetUpdateCallback(playdate.SpriteUpdateCallback) error {
	return s.record("updateCallback")
}
func (s *testSprite) SetCollisionResponseCallback(playdate.SpriteCollisionResponseCallback) error {
	return s.record("collisionCallback")
}
func (s *testSprite) Add() error    { return s.record("add") }
func (s *testSprite) Remove() error { return s.record("remove") }
func (s *testSprite) Close() error  { return s.record("close") }

type testContext struct {
	operations []string
	input      playdate.Input
	create     int
	failAt     int
}

func (*testContext) Clear()                                               {}
func (*testContext) DrawText(string, int, int)                            {}
func (*testContext) CurrentTimeMilliseconds() uint32                      { return 0 }
func (c *testContext) Input() playdate.Input                              { return c.input }
func (c *testContext) LoadBitmap(string) (playdate.Bitmap, error)         { return nil, nil }
func (*testContext) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (c *testContext) NewBitmap(int, int) (playdate.Bitmap, error) {
	return &testBitmap{&c.operations}, nil
}
func (*testContext) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (*testContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (c *testContext) DrawInto(_ playdate.Bitmap, callback func() error) error {
	c.operations = append(c.operations, "drawInto")
	return callback()
}
func (c *testContext) FillRect(int, int, int, int, playdate.Paint) error {
	c.operations = append(c.operations, "fillRect")
	return nil
}
func (c *testContext) NewSprite() (playdate.Sprite, error) {
	c.create++
	if c.create == c.failAt {
		return nil, errors.New("create failed")
	}
	return &testSprite{name: string(rune('0' + c.create)), operations: &c.operations}, nil
}
func (*testContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite { return nil }
func (*testContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite     { return nil }
func (*testContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (c *testContext) UpdateAndDrawSprites() { c.operations = append(c.operations, "draw") }
func (c *testContext) SetAlwaysRedraw(value bool) {
	c.operations = append(c.operations, "redraw."+map[bool]string{false: "dirty", true: "full"}[value])
}
func (c *testContext) AddDirtyRect(int, int, int, int) error {
	c.operations = append(c.operations, "screenDirty")
	return nil
}
func (c *testContext) SetRefreshRate(float32) error {
	c.operations = append(c.operations, "refresh")
	return nil
}
func (*testContext) Width() int            { return 400 }
func (*testContext) Height() int           { return 240 }
func (*testContext) RefreshRate() float32  { return 30 }
func (*testContext) FPS() float32          { return 29.5 }
func (c *testContext) SetInverted(bool)    { c.operations = append(c.operations, "inverted") }
func (c *testContext) SetScale(uint) error { c.operations = append(c.operations, "scale"); return nil }
func (c *testContext) SetMosaic(uint, uint) error {
	c.operations = append(c.operations, "mosaic")
	return nil
}
func (c *testContext) SetFlipped(bool, bool) { c.operations = append(c.operations, "flipped") }
func (c *testContext) SetOffset(int, int)    { c.operations = append(c.operations, "offset") }

func TestAcceptanceMovesCrankSpriteAndDrawsDisplayList(t *testing.T) {
	context := &testContext{input: playdate.Input{CrankDelta: 4}}
	game := New().(*game)
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	context.operations = nil
	if refresh, err := game.Update(context); err != nil || !refresh {
		t.Fatalf("Update() = %t, %v", refresh, err)
	}
	want := []string{"1.position", "2.move", "draw"}
	if !reflect.DeepEqual(context.operations, want) {
		t.Fatalf("operations = %v, want %v", context.operations, want)
	}
}

func TestAcceptanceTogglesFullRedrawAndDisplayPresentation(t *testing.T) {
	context := &testContext{input: playdate.Input{Pressed: playdate.ButtonA | playdate.ButtonB}}
	game := New().(*game)
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	context.operations = nil
	if _, err := game.Update(context); err != nil {
		t.Fatal(err)
	}
	wantPrefix := []string{"redraw.full", "inverted", "flipped", "offset", "scale", "mosaic", "inverted"}
	if len(context.operations) < len(wantPrefix) || !reflect.DeepEqual(context.operations[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("operations = %v, want prefix %v", context.operations, wantPrefix)
	}
	if !game.fullRedraw || game.presentation != 1 {
		t.Fatalf("state = full:%t presentation:%d", game.fullRedraw, game.presentation)
	}
}

func TestAcceptanceMarksAlternatingPartialSpriteRegions(t *testing.T) {
	context := &testContext{}
	game := New().(*game)
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	context.operations = nil
	game.frame = pulseFrames - 1
	if _, err := game.Update(context); err != nil {
		t.Fatal(err)
	}
	want := []string{"1.position", "2.move", "drawInto", "fillRect", "3.dirtyRect", "screenDirty", "draw"}
	if !reflect.DeepEqual(context.operations, want) {
		t.Fatalf("operations = %v, want %v", context.operations, want)
	}
}

func TestInitializationRollbackClosesSpritesBeforeBitmap(t *testing.T) {
	context := &testContext{failAt: 3}
	err := New().Init(context)
	if err == nil {
		t.Fatal("Init() succeeded")
	}
	wantTail := []string{"1.close", "2.close", "bitmap.close"}
	got := context.operations[len(context.operations)-len(wantTail):]
	if !reflect.DeepEqual(got, wantTail) {
		t.Fatalf("rollback tail = %v, want %v", got, wantTail)
	}
}
