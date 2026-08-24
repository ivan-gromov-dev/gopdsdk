package runtime

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

const callbackAudioBlockFrames = 512

// PCMCallbackSourceDriver bridges one bounded native PCM ring.
type PCMCallbackSourceDriver struct {
	Audio     AudioDriver
	New       func(channel uintptr, stereo bool) uintptr
	Available func(uintptr) int
	Write     func(uintptr, []int16, []int16) int
	Underruns func(uintptr) uint32
}

type pcmCallbackSource struct {
	*audioPlayer
	driver      PCMCallbackSourceDriver
	callback    playdate.PCMRenderCallback
	stereo      bool
	left, right [callbackAudioBlockFrames]int16
}

var pcmCallbackSources = map[*pcmCallbackSource]struct{}{}

// NewPCMCallbackSource creates a source already attached to channel.
func NewPCMCallbackSource(channel playdate.AudioChannel, stereo bool, callback playdate.PCMRenderCallback, driver PCMCallbackSourceDriver) (playdate.PCMCallbackSource, error) {
	if callback == nil {
		return nil, playdate.ErrAudioCallback
	}
	c, ok := channel.(*audioChannel)
	if !ok {
		return nil, playdate.ErrAudioSourceInvalid
	}
	ch, err := c.nativeHandle()
	if err != nil {
		return nil, err
	}
	h := driver.New(ch, stereo)
	if h == 0 {
		return nil, playdate.ErrAudioCreate
	}
	p := &pcmCallbackSource{
		audioPlayer: &audioPlayer{handle: h, driver: driver.Audio, channels: map[*audioChannel]struct{}{c: {}}},
		driver:      driver,
		callback:    callback,
		stereo:      stereo,
	}
	c.sources[p.audioPlayer] = struct{}{}
	pcmCallbackSources[p] = struct{}{}
	return p, nil
}

// RefillPCMCallbackSources runs render callbacks on the update goroutine.
func RefillPCMCallbackSources() {
	for source := range pcmCallbackSources {
		if source.closed {
			continue
		}
		available := source.driver.Available(source.handle)
		for available > 0 {
			n := min(available, callbackAudioBlockFrames)
			left := source.left[:n]
			var right []int16
			if source.stereo {
				right = source.right[:n]
			}
			produced := source.callback(left, right)
			produced = max(0, min(produced, n))
			if produced == 0 {
				break
			}
			if right != nil {
				right = right[:produced]
			}
			written := source.driver.Write(source.handle, left[:produced], right)
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

func (s *pcmCallbackSource) UnderrunCount() (uint32, error) {
	handle, err := s.nativeHandle()
	if err != nil {
		return 0, err
	}
	return s.driver.Underruns(handle), nil
}

func (s *pcmCallbackSource) Close() error {
	if s == nil {
		return playdate.ErrAudioClosed
	}
	if err := s.audioPlayer.Close(); err != nil {
		return err
	}
	delete(pcmCallbackSources, s)
	s.callback = nil
	return nil
}
