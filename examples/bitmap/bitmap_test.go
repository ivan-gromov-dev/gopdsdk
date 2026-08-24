package bitmap

import (
	"reflect"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testBitmap struct {
	width, height int
	operations    *[]string
	name          string
}

func (b *testBitmap) Width() (int, error)  { return b.width, nil }
func (b *testBitmap) Height() (int, error) { return b.height, nil }
func (b *testBitmap) Clear() error         { return b.Fill(playdate.ColorClear) }
func (b *testBitmap) Fill(color playdate.Color) error {
	*b.operations = append(*b.operations, b.name+".fill:"+string(rune('0'+color)))
	return nil
}
func (b *testBitmap) Close() error {
	*b.operations = append(*b.operations, b.name+".close")
	return nil
}

type testContext struct {
	operations []string
	loaded     *testBitmap
	created    *testBitmap
}

func newTestContext() *testContext {
	context := &testContext{}
	context.loaded = &testBitmap{width: 64, height: 64, operations: &context.operations, name: "loaded"}
	context.created = &testBitmap{width: 48, height: 48, operations: &context.operations, name: "created"}
	return context
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
func (*testContext) UpdateAndDrawSprites() {}
func (c *testContext) LoadBitmap(path string) (playdate.Bitmap, error) {
	c.operations = append(c.operations, "load:"+path)
	return c.loaded, nil
}
func (*testContext) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (c *testContext) NewBitmap(width, height int) (playdate.Bitmap, error) {
	c.operations = append(c.operations, "new:48x48")
	return c.created, nil
}
func (c *testContext) DrawBitmap(bitmap playdate.Bitmap, x, y int) error {
	name := "created"
	if bitmap == c.loaded {
		name = "loaded"
	}
	c.operations = append(c.operations, "draw:"+name)
	return nil
}
func (c *testContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error {
	c.operations = append(c.operations, "draw:scaled")
	return nil
}

func TestAcceptanceLifecycle(t *testing.T) {
	context := newTestContext()
	probe := New().(*game)
	if err := probe.Init(context); err != nil {
		t.Fatal(err)
	}
	if refresh, err := probe.Update(context); err != nil || !refresh {
		t.Fatalf("Update() = %t, %v", refresh, err)
	}
	if err := probe.HandleLifecycle(context, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if err := probe.HandleLifecycle(context, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	want := []string{"load:" + assetPath, "new:48x48", "created.fill:2", "draw:loaded", "draw:scaled", "draw:created", "loaded.close", "created.close"}
	if !reflect.DeepEqual(context.operations, want) {
		t.Fatalf("operations = %v, want %v", context.operations, want)
	}
}
