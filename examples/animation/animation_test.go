package animation

import (
	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	"image/png"
	"os"
	"testing"
)

type testBitmap struct{ index int }

func (*testBitmap) Width() (int, error)       { return 16, nil }
func (*testBitmap) Height() (int, error)      { return 16, nil }
func (*testBitmap) Clear() error              { return nil }
func (*testBitmap) Fill(playdate.Color) error { return nil }
func (*testBitmap) Close() error              { return playdate.ErrBitmapBorrowed }

type testTable struct{ closed bool }

func (*testTable) Frame(index int) (playdate.Bitmap, error) { return &testBitmap{index: index}, nil }
func (t *testTable) Close() error                           { t.closed = true; return nil }

type testContext struct {
	table *testTable
	loads int
	input playdate.Input
	draws []int
}

func (*testContext) CurrentTimeMilliseconds() uint32            { return 0 }
func (c *testContext) Input() playdate.Input                    { return c.input }
func (*testContext) Clear()                                     {}
func (*testContext) DrawText(string, int, int)                  {}
func (*testContext) LoadBitmap(string) (playdate.Bitmap, error) { return nil, nil }
func (c *testContext) LoadBitmapTable(path string) (playdate.BitmapTable, error) {
	if path != tableAsset {
		panic(path)
	}
	c.loads++
	c.table = &testTable{}
	return c.table, nil
}
func (*testContext) NewBitmap(int, int) (playdate.Bitmap, error) { return nil, nil }
func (c *testContext) DrawBitmap(value playdate.Bitmap, _ int, _ int) error {
	c.draws = append(c.draws, value.(*testBitmap).index)
	return nil
}
func (*testContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*testContext) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*testContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*testContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*testContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (*testContext) UpdateAndDrawSprites() {}

func TestPlayerAndObstaclesShareOneTable(t *testing.T) {
	c := &testContext{input: playdate.Input{DeltaSeconds: .13}}
	g := New().(*game)
	if err := g.Init(c); err != nil {
		t.Fatal(err)
	}
	if c.loads != 1 {
		t.Fatalf("table loads = %d", c.loads)
	}
	for range 4 {
		if _, err := g.Update(c); err != nil {
			t.Fatal(err)
		}
	}
	for update := range 4 {
		got := c.draws[update*3 : update*3+3]
		if got[0] > 1 || got[1] != 2 || got[2] != 3 {
			t.Fatalf("update %d draws = %v; player must use 0..1 and obstacles 2,3", update, got)
		}
	}
	if err := g.HandleLifecycle(c, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if !c.table.closed {
		t.Fatal("table was not closed")
	}
}

func TestCharacterTableContainsFourVisibleDistinctFrames(t *testing.T) {
	file, err := os.Open("resources/images/characters-table-32-32.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	image, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := image.Bounds().Size(); got.X != 128 || got.Y != 32 {
		t.Fatalf("table size = %dx%d, want 128x32", got.X, got.Y)
	}
	var signatures [4][128]byte
	for frame := range signatures {
		black := 0
		for y := range 32 {
			for x := range 32 {
				r, _, _, _ := image.At(frame*32+x, y).RGBA()
				if r < 0x8000 {
					black++
					signatures[frame][y*4+x/8] |= 1 << (x % 8)
				}
			}
		}
		if black < 24 {
			t.Fatalf("frame %d has only %d black pixels", frame, black)
		}
	}
	for left := range signatures {
		for right := left + 1; right < len(signatures); right++ {
			if signatures[left] == signatures[right] {
				t.Fatalf("frames %d and %d are identical", left, right)
			}
		}
	}
}
