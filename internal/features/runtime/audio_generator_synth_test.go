package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestGeneratorSynthRefillsIndependentVoiceRings(t *testing.T) {
	generatorSynthSources = make(map[*generatorSynthSource]struct{})
	available := map[uintptr]int{11: 300, 12: 40}
	var parameter int
	var states []playdate.GeneratorState
	var writes [][2]int
	driver := GeneratorSynthDriver{
		Synth: SynthDriver{Audio: AudioDriver{
			Stop: func(uintptr) {}, Free: func(uintptr) {},
			SetVolume: func(uintptr, float32, float32) {},
			Volume:    func(uintptr) (float32, float32) { return 1, 1 },
			IsPlaying: func(uintptr) bool { return true }, Pause: func(uintptr, bool) {},
		}},
		New: func(stereo bool) uintptr {
			if !stereo {
				t.Fatal("New received mono, want stereo")
			}
			return 7
		},
		Voices: func(root uintptr, dst []GeneratorVoiceState) int {
			if root != 7 {
				t.Fatalf("root = %d", root)
			}
			dst[0] = GeneratorVoiceState{Handle: 11, State: playdate.GeneratorState{Voice: 1, Note: 60, Velocity: .8, Rate: 1, Parameters: [8]float32{.25}}}
			dst[1] = GeneratorVoiceState{Handle: 12, State: playdate.GeneratorState{Voice: 2, Note: 67, Velocity: .7, Rate: 1, Released: true, ReleaseOffset: 17}}
			return 2
		},
		Available: func(voice uintptr) int { return available[voice] },
		Write: func(voice uintptr, left, right []int16) int {
			if len(right) != len(left) {
				t.Fatalf("stereo lengths = %d,%d", len(left), len(right))
			}
			writes = append(writes, [2]int{int(voice), len(left)})
			available[voice] -= len(left)
			return len(left)
		},
		SetParameter: func(_ uintptr, index int, _ float32) bool { parameter = index; return true },
	}
	synth, err := NewGeneratorSynth(true, func(state playdate.GeneratorState, left, right []int16) int {
		states = append(states, state)
		return len(left)
	}, driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := synth.SetParameter(0, .5); !errors.Is(err, playdate.ErrAudioParameter) {
		t.Fatalf("zero-based parameter error = %v", err)
	}
	if err := synth.SetParameter(1, .5); err != nil || parameter != 1 {
		t.Fatalf("SetParameter(1) = %v, native index %d", err, parameter)
	}
	if err := synth.SetWaveform(playdate.WaveformSine); !errors.Is(err, playdate.ErrAudioUnavailable) {
		t.Fatalf("generator SetWaveform error = %v", err)
	}
	if err := synth.SetWavetable(nil, 8, 1, 1); !errors.Is(err, playdate.ErrAudioUnavailable) {
		t.Fatalf("generator SetWavetable error = %v", err)
	}
	RefillGeneratorSynths()
	if len(states) != 3 || states[0].Note != 60 || states[2].Note != 67 || !states[2].Released || states[2].ReleaseOffset != 17 {
		t.Fatalf("states = %#v", states)
	}
	want := [][2]int{{11, 256}, {11, 44}, {12, 40}}
	if len(writes) != len(want) {
		t.Fatalf("writes = %v", writes)
	}
	for i := range want {
		if writes[i] != want[i] {
			t.Fatalf("writes = %v, want %v", writes, want)
		}
	}
	if err := synth.Close(); err != nil {
		t.Fatal(err)
	}
	available[11] = 10
	RefillGeneratorSynths()
	if len(states) != 3 {
		t.Fatal("generator callback ran after Close")
	}
}

func TestGeneratorSynthRejectsInvalidCallbackAndCreation(t *testing.T) {
	if _, err := NewGeneratorSynth(false, nil, GeneratorSynthDriver{}); !errors.Is(err, playdate.ErrAudioCallback) {
		t.Fatalf("nil callback error = %v", err)
	}
	if _, err := NewGeneratorSynth(false, func(playdate.GeneratorState, []int16, []int16) int { return 0 }, GeneratorSynthDriver{New: func(bool) uintptr { return 0 }}); !errors.Is(err, playdate.ErrAudioCreate) {
		t.Fatalf("create error = %v", err)
	}
}
