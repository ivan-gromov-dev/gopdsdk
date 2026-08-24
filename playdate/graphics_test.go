package playdate_test

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestPaintRepresentations(t *testing.T) {
	white, err := playdate.SolidPaint(playdate.ColorWhite)
	if err != nil {
		t.Fatal(err)
	}
	if solid, _, patterned := white.Components(); solid != 1 || patterned {
		t.Fatalf("white components = %d, %v", solid, patterned)
	}
	if _, err := playdate.SolidPaint(playdate.Color(99)); !errors.Is(err, playdate.ErrGraphicsColor) {
		t.Fatalf("invalid solid paint = %v", err)
	}
	if solid, _, patterned := playdate.XORPaint().Components(); solid != 3 || patterned {
		t.Fatalf("XOR components = %d, %v", solid, patterned)
	}

	image := [8]byte{0x80, 0x40, 0x20, 0x10, 8, 4, 2, 1}
	mask := [8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	paint := playdate.PatternPaint(image, mask)
	image[0] = 0
	_, pattern, patterned := paint.Components()
	if !patterned || pattern[0] != 0x80 || pattern[8] != 0xff {
		t.Fatalf("pattern components = %x, %v", pattern, patterned)
	}
}
