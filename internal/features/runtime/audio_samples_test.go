package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestAudioSampleBorrowingAndPlayerOwnership(t *testing.T) {
	freed := 0
	native := []byte{1, 2, 3, 4}
	sample := NewAudioSample(7, AudioSampleDriver{
		Load:       func(uintptr, string) bool { return true },
		Data:       func(uintptr) ([]byte, playdate.SoundFormat, uint32) { return native, playdate.Sound16BitMono, 22050 },
		Length:     func(uintptr) float32 { return .25 },
		Decompress: func(uintptr) bool { return true },
		Free:       func(uintptr) { freed++ },
	})
	var attached uintptr
	var ranges [][2]int
	player := NewSamplePlayer(9, AudioDriver{
		SetSample:       func(_, value uintptr) { attached = value },
		SetPlayRange:    func(_ uintptr, start, end int) { ranges = append(ranges, [2]int{start, end}) },
		SetLoopCallback: func(uintptr, uint32) {}, Stop: func(uintptr) {}, Free: func(uintptr) {},
	})
	controls := player.(playdate.SamplePlayerControls)
	if err := controls.SetSample(sample); err != nil || attached != 7 {
		t.Fatalf("SetSample = %v, handle %d", err, attached)
	}
	if err := sample.Close(); !errors.Is(err, playdate.ErrAudioSampleInUse) {
		t.Fatalf("attached Close = %v", err)
	}
	if err := controls.SetPlayRange(2, 4); err != nil || len(ranges) != 1 {
		t.Fatalf("SetPlayRange = %v, %v", err, ranges)
	}
	view, err := sample.Data()
	if err != nil {
		t.Fatal(err)
	}
	dst := make([]byte, 3)
	if n, err := view.CopyTo(dst); err != nil || n != 3 || dst[2] != 3 {
		t.Fatalf("CopyTo = %d,%v,%v", n, err, dst)
	}
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sample.Close(); err != nil || freed != 1 {
		t.Fatalf("Close = %v, frees %d", err, freed)
	}
	if _, err := view.CopyTo(dst); !errors.Is(err, playdate.ErrAudioSampleClosed) {
		t.Fatalf("stale view = %v", err)
	}
}

func TestFilePlayerBufferLoopAndUnderrun(t *testing.T) {
	var buffer, start, end float32
	var stop bool
	p := NewFilePlayer(3, AudioDriver{Load: func(uintptr, string) bool { return true }, SetBufferLength: func(_ uintptr, v float32) { buffer = v }, SetLoopRange: func(_ uintptr, s, e float32) { start, end = s, e }, DidUnderrun: func(uintptr) bool { return true }, SetStopOnUnderrun: func(_ uintptr, v bool) { stop = v }, SetLoopCallback: func(uintptr, uint32) {}, Stop: func(uintptr) {}, Free: func(uintptr) {}})
	controls := p.(playdate.StreamingPlayerControls)
	if err := controls.Load("audio/music"); err != nil {
		t.Fatal(err)
	}
	if err := controls.SetBufferLength(.5); err != nil {
		t.Fatal(err)
	}
	if err := controls.SetLoopRange(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := controls.SetStopOnUnderrun(true); err != nil {
		t.Fatal(err)
	}
	underrun, err := controls.DidUnderrun()
	if err != nil || !underrun || buffer != .5 || start != 1 || end != 2 || !stop {
		t.Fatalf("state = %v,%v,%v,%v,%v,%v", underrun, err, buffer, start, end, stop)
	}
}
