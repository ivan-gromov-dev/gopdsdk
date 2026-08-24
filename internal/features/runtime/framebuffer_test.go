package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestFramebufferPixelsAndDirtyRows(t *testing.T) {
	data := make([]byte, 12)
	var markedStart, markedEnd = -1, -1
	err := WithFramebuffer(data, 10, 3, 4, func(start, end int) { markedStart, markedEnd = start, end }, func(frame playdate.Framebuffer) error {
		if frame.Width() != 10 || frame.Height() != 3 || frame.RowBytes() != 4 {
			t.Fatalf("unexpected geometry")
		}
		if err := frame.SetPixel(0, 2, playdate.ColorWhite); err != nil {
			return err
		}
		if err := frame.SetPixel(9, 0, playdate.ColorWhite); err != nil {
			return err
		}
		if color, err := frame.Pixel(9, 0); err != nil || color != playdate.ColorWhite {
			t.Fatalf("pixel = %v, %v", color, err)
		}
		if err := frame.SetPixel(9, 0, playdate.ColorBlack); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if data[8] != 0x80 || data[1] != 0 {
		t.Fatalf("data = % x", data)
	}
	if markedStart != 0 || markedEnd != 2 {
		t.Fatalf("dirty rows = %d..%d", markedStart, markedEnd)
	}
}

func TestFramebufferLifetimeAndValidation(t *testing.T) {
	var retained playdate.Framebuffer
	err := WithFramebuffer(make([]byte, 4), 8, 1, 4, nil, func(frame playdate.Framebuffer) error {
		retained = frame
		if err := frame.SetPixel(-1, 0, playdate.ColorWhite); !errors.Is(err, playdate.ErrFramebufferBounds) {
			t.Fatalf("bounds: %v", err)
		}
		if err := frame.SetPixel(0, 0, playdate.ColorClear); !errors.Is(err, playdate.ErrFramebufferColor) {
			t.Fatalf("color: %v", err)
		}
		return frame.MarkDirtyRows(0, 1)
	})
	if !errors.Is(err, playdate.ErrFramebufferBounds) {
		t.Fatalf("callback error: %v", err)
	}
	if _, err := retained.Bytes(); !errors.Is(err, playdate.ErrFramebufferExpired) {
		t.Fatalf("retained: %v", err)
	}
	if err := WithFramebuffer(nil, 0, 0, 0, nil, nil); !errors.Is(err, playdate.ErrFramebufferCallback) {
		t.Fatalf("nil callback: %v", err)
	}
}
