package oomprobe

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type testContext struct{}

func (testContext) Clear()                                                             {}
func (testContext) DrawText(string, int, int)                                          {}
func (testContext) CurrentTimeMilliseconds() uint32                                    { return 0 }
func (testContext) Input() playdate.Input                                              { return playdate.Input{} }
func (testContext) LoadBitmap(string) (playdate.Bitmap, error)                         { return nil, nil }
func (testContext) LoadBitmapTable(string) (playdate.BitmapTable, error)               { return nil, nil }
func (testContext) NewBitmap(int, int) (playdate.Bitmap, error)                        { return nil, nil }
func (testContext) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (testContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (testContext) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (testContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (testContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (testContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (testContext) UpdateAndDrawSprites() {}

func TestUpdateRetainsEveryAllocatedBlock(t *testing.T) {
	game := &game{}
	const frames = 4
	for range frames {
		refresh, err := game.Update(testContext{})
		if err != nil || !refresh {
			t.Fatalf("Update() = %v, %v; want true, nil", refresh, err)
		}
	}
	if game.frame != frames {
		t.Fatalf("frame = %d, want %d", game.frame, frames)
	}
	for index := range frames {
		if len(game.blocks[index]) != blockSize {
			t.Fatalf("blocks[%d] length = %d, want %d", index, len(game.blocks[index]), blockSize)
		}
	}
}
