package runtime

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

const generatorBlockFrames = 256

// GeneratorVoiceState is one native voice snapshot returned outside the audio
// callback. Handle is a bounded native voice slot, including copied voices.
type GeneratorVoiceState struct {
	Handle uintptr
	State  playdate.GeneratorState
}

// GeneratorSynthDriver bridges fixed native voice rings and event snapshots.
type GeneratorSynthDriver struct {
	Synth        SynthDriver
	New          func(stereo bool) uintptr
	Voices       func(root uintptr, dst []GeneratorVoiceState) int
	Available    func(voice uintptr) int
	Write        func(voice uintptr, left, right []int16) int
	SetParameter func(root uintptr, parameter int, value float32) bool
}

type generatorSynthSource struct {
	root     uintptr
	driver   GeneratorSynthDriver
	callback playdate.GeneratorRenderCallback
	stereo   bool
	voices   [8]GeneratorVoiceState
	left     [generatorBlockFrames]int16
	right    [generatorBlockFrames]int16
}

var generatorSynthSources = map[*generatorSynthSource]struct{}{}

// NewGeneratorSynth creates a native synth whose audio callback only consumes
// fixed native rings. Rendering and native event inspection happen in Update.
func NewGeneratorSynth(stereo bool, callback playdate.GeneratorRenderCallback, driver GeneratorSynthDriver) (playdate.GeneratorSynth, error) {
	if callback == nil {
		return nil, playdate.ErrAudioCallback
	}
	handle := driver.New(stereo)
	if handle == 0 {
		return nil, playdate.ErrAudioCreate
	}
	value := &generatorSynthSource{root: handle, driver: driver, callback: callback, stereo: stereo}
	synthDriver := driver.Synth
	synthDriver.SetParameter = driver.SetParameter
	s := NewSynth(handle, synthDriver).(*synth)
	s.generator = value
	generatorSynthSources[value] = struct{}{}
	return s, nil
}

// RefillGeneratorSynths fills every live root or copied native synth voice.
func RefillGeneratorSynths() {
	for source := range generatorSynthSources {
		count := source.driver.Voices(source.root, source.voices[:])
		count = max(0, min(count, len(source.voices)))
		for index := 0; index < count; index++ {
			voice := source.voices[index]
			if voice.State.Velocity == 0 && voice.State.Rate == 0 {
				continue
			}
			available := source.driver.Available(voice.Handle)
			for available > 0 {
				n := min(available, generatorBlockFrames)
				left := source.left[:n]
				var right []int16
				if source.stereo {
					right = source.right[:n]
				}
				produced := max(0, min(source.callback(voice.State, left, right), n))
				if produced == 0 {
					break
				}
				if right != nil {
					right = right[:produced]
				}
				written := source.driver.Write(voice.Handle, left[:produced], right)
				if written <= 0 {
					break
				}
				available -= written
				if written < produced {
					break
				}
			}
		}
	}
}
