package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestPCMCallbackSourceRefillsBoundedBlocksAndCloses(t *testing.T) {
	pcmCallbackSources = make(map[*pcmCallbackSource]struct{})
	available := 700
	var calls, freed int
	var blocks []int
	driver := PCMCallbackSourceDriver{
		Audio: AudioDriver{
			Source: func(handle uintptr) uintptr { return handle + 1 },
			Stop:   func(uintptr) {}, SetVolume: func(uintptr, float32, float32) {},
			Volume:    func(uintptr) (float32, float32) { return .5, .75 },
			IsPlaying: func(uintptr) bool { return true }, Pause: func(uintptr, bool) {},
			Free: func(uintptr) { freed++ },
		},
		New:       func(channel uintptr, stereo bool) uintptr { return channel + 10 },
		Available: func(uintptr) int { return available },
		Write: func(_ uintptr, left, right []int16) int {
			blocks = append(blocks, len(left))
			if right != nil {
				t.Fatal("mono callback source wrote a right channel")
			}
			available -= len(left)
			return len(left)
		},
		Underruns: func(uintptr) uint32 { return 3 },
	}
	channel := NewAudioChannel(5, AudioChannelDriver{RemoveSource: func(uintptr, uintptr) bool { return true }}).(playdate.AudioChannel)
	source, err := NewPCMCallbackSource(channel, false, func(left, right []int16) int {
		calls++
		if right != nil {
			t.Fatal("mono callback received a right channel")
		}
		return len(left)
	}, driver)
	if err != nil {
		t.Fatal(err)
	}
	RefillPCMCallbackSources()
	if calls != 2 || len(blocks) != 2 || blocks[0] != 512 || blocks[1] != 188 {
		t.Fatalf("refill calls=%d blocks=%v, want [512 188]", calls, blocks)
	}
	if count, err := source.UnderrunCount(); err != nil || count != 3 {
		t.Fatalf("UnderrunCount() = %d, %v", count, err)
	}
	if err := source.Close(); err != nil || freed != 1 {
		t.Fatalf("Close() = %v, frees=%d", err, freed)
	}
	available = 512
	RefillPCMCallbackSources()
	if calls != 2 {
		t.Fatalf("callback ran after Close: %d calls", calls)
	}
	if _, err := source.UnderrunCount(); !errors.Is(err, playdate.ErrAudioClosed) {
		t.Fatalf("closed UnderrunCount() error = %v", err)
	}
}

func TestPCMCallbackSourceValidatesAndClampsCallbackCount(t *testing.T) {
	pcmCallbackSources = make(map[*pcmCallbackSource]struct{})
	driver := PCMCallbackSourceDriver{
		Audio: AudioDriver{Source: func(h uintptr) uintptr { return h }, Stop: func(uintptr) {}, Free: func(uintptr) {}},
		New:   func(uintptr, bool) uintptr { return 9 }, Available: func(uintptr) int { return 16 },
		Write: func(_ uintptr, left, right []int16) int {
			if len(left) != 16 || len(right) != 16 {
				t.Fatalf("Write lengths = %d,%d, want 16,16", len(left), len(right))
			}
			return len(left)
		},
	}
	channel := NewAudioChannel(1, AudioChannelDriver{RemoveSource: func(uintptr, uintptr) bool { return true }}).(playdate.AudioChannel)
	if _, err := NewPCMCallbackSource(channel, false, nil, driver); !errors.Is(err, playdate.ErrAudioCallback) {
		t.Fatalf("nil callback error = %v", err)
	}
	if _, err := NewPCMCallbackSource(nil, false, func([]int16, []int16) int { return 0 }, driver); !errors.Is(err, playdate.ErrAudioSourceInvalid) {
		t.Fatalf("invalid channel error = %v", err)
	}
	source, err := NewPCMCallbackSource(channel, true, func(left, right []int16) int { return len(left) + 100 }, driver)
	if err != nil {
		t.Fatal(err)
	}
	RefillPCMCallbackSources()
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPCMCallbackSourceSurvivesChannelFirstClose(t *testing.T) {
	pcmCallbackSources = make(map[*pcmCallbackSource]struct{})
	var removed, sourceFreed, channelFreed int
	channel := NewAudioChannel(2, AudioChannelDriver{
		RemoveSource: func(channel, source uintptr) bool {
			removed++
			if channel != 2 || source != 30 {
				t.Fatalf("RemoveSource(%d,%d)", channel, source)
			}
			return true
		},
		Remove: func(uintptr) bool { return true }, Free: func(uintptr) { channelFreed++ },
	})
	source, err := NewPCMCallbackSource(channel, false, func([]int16, []int16) int { return 0 }, PCMCallbackSourceDriver{
		Audio: AudioDriver{Source: func(uintptr) uintptr { return 30 }, Stop: func(uintptr) {}, Free: func(uintptr) { sourceFreed++ }},
		New:   func(uintptr, bool) uintptr { return 3 },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := channel.Close(); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if removed != 1 || sourceFreed != 1 || channelFreed != 1 {
		t.Fatalf("removed=%d source frees=%d channel frees=%d", removed, sourceFreed, channelFreed)
	}
}
