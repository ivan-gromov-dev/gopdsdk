// Package microphone exercises P5.5 permission and bounded audio input.
package microphone

import (
	"errors"
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	recordingFile = "microphone.wav"
	sampleRate    = 44100
)

type game struct {
	microphones playdate.Microphones
	pcmPlayers  playdate.PCMPlayers
	files       playdate.FileSystem
	recorder    playdate.MicrophoneRecorder
	playback    playdate.SoundEffect
	playPending bool
	permission  playdate.MicrophonePermission
	capture     [sampleRate]int16
	recorded    int
	saved       int
	peak        uint32
	blocks      uint32
	err         error
	dirty       bool
	closed      bool
}

// New creates the P5.5 microphone acceptance game.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	microphones, ok := context.(playdate.Microphones)
	if !ok {
		return playdate.ErrMicrophoneUnavailable
	}
	g.microphones = microphones
	pcmPlayers, ok := context.(playdate.PCMPlayers)
	if !ok {
		return playdate.ErrAudioUnavailable
	}
	g.pcmPlayers = pcmPlayers
	files, ok := context.(playdate.FileSystem)
	if !ok {
		return playdate.ErrFileUnavailable
	}
	g.files = files
	g.dirty = true
	permission, err := microphones.RequestMicrophoneAccess("show live microphone level", g.permissionChanged)
	g.permission = permission
	return err
}

func (g *game) permissionChanged(permission playdate.MicrophonePermission) {
	g.permission = permission
	g.dirty = true
	if permission == playdate.MicrophonePermissionGranted && g.recorder == nil && !g.closed {
		g.start()
	}
}

func (g *game) start() {
	g.recorded = 0
	g.saved = 0
	g.recorder, g.err = g.microphones.StartMicrophoneRecording(playdate.MicrophoneSourceAutomatic, g.samples)
	g.dirty = true
}

func (g *game) stop() {
	if g.recorder != nil {
		g.err = g.recorder.Close()
		g.recorder = nil
	}
	g.dirty = true
}

func (g *game) samples(samples playdate.MicrophoneSamples) bool {
	remaining := g.capture[g.recorded:]
	count, err := samples.CopyTo(remaining)
	if err != nil {
		g.err = err
		return false
	}
	var peak uint32
	for _, sample := range remaining[:count] {
		value := int32(sample)
		if value < 0 {
			value = -value
		}
		if uint32(value) > peak {
			peak = uint32(value)
		}
	}
	g.peak = peak
	g.recorded += count
	g.blocks++
	g.dirty = true
	if g.recorded == len(g.capture) {
		g.recorder = nil
		return false
	}
	return !g.closed
}

func (g *game) Update(context playdate.Context) (bool, error) {
	if g.playPending {
		g.playPending = false
		g.err = g.startPlayback()
		g.dirty = true
	}
	if context.Input().Pressed.Has(playdate.ButtonA) && g.permission == playdate.MicrophonePermissionGranted {
		if g.recorder == nil {
			g.start()
		} else {
			g.stop()
		}
	}
	if context.Input().Pressed.Has(playdate.ButtonB) && g.recorded > 0 {
		g.stop()
		g.err = g.save()
	}
	if !g.dirty {
		return false, nil
	}
	g.dirty = false
	context.Clear()
	context.DrawText("P5.5 microphone input", 12, 16)
	context.DrawText("Permission: "+permissionName(g.permission), 12, 52)
	context.DrawText("Recording: "+strconv.FormatBool(g.recorder != nil), 12, 82)
	context.DrawText("Peak: "+strconv.FormatUint(uint64(g.peak), 10), 12, 112)
	context.DrawText("Samples: "+strconv.Itoa(g.recorded)+" Saved:"+strconv.Itoa(g.saved), 12, 142)
	context.DrawText("A: record  B: save/play", 12, 178)
	if g.err != nil {
		context.DrawText("Error: "+g.err.Error(), 12, 208)
	}
	return true, nil
}

func (g *game) save() error {
	if g.playback != nil {
		if err := g.playback.Close(); err != nil {
			return err
		}
		g.playback = nil
	}
	if err := g.writeWAV(); err != nil {
		return err
	}
	g.saved = g.recorded
	g.playPending = true
	g.dirty = true
	return nil
}

func (g *game) startPlayback() error {
	player, err := g.pcmPlayers.NewPCMPlayer(g.capture[:g.recorded], sampleRate)
	if err != nil {
		return err
	}
	g.playback = player
	if err = player.Play(); err != nil {
		g.playback = nil
		return errors.Join(err, player.Close())
	}
	return nil
}

func (g *game) writeWAV() error {
	file, err := g.files.OpenFile(recordingFile, playdate.FileWrite)
	if err != nil {
		return err
	}
	dataBytes := uint32(g.recorded * 2)
	var header [44]byte
	copy(header[0:4], "RIFF")
	putUint32(header[4:8], 36+dataBytes)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	putUint32(header[16:20], 16)
	putUint16(header[20:22], 1)
	putUint16(header[22:24], 1)
	putUint32(header[24:28], sampleRate)
	putUint32(header[28:32], sampleRate*2)
	putUint16(header[32:34], 2)
	putUint16(header[34:36], 16)
	copy(header[36:40], "data")
	putUint32(header[40:44], dataBytes)
	if _, err = file.Write(header[:]); err != nil {
		return errors.Join(err, file.Close())
	}
	var encoded [512]byte
	for offset := 0; offset < g.recorded; {
		count := min(len(encoded)/2, g.recorded-offset)
		for index, sample := range g.capture[offset : offset+count] {
			putUint16(encoded[index*2:index*2+2], uint16(sample))
		}
		if _, err = file.Write(encoded[:count*2]); err != nil {
			return errors.Join(err, file.Close())
		}
		offset += count
	}
	if err = file.Flush(); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}

func putUint16(destination []byte, value uint16) {
	destination[0], destination[1] = byte(value), byte(value>>8)
}

func putUint32(destination []byte, value uint32) {
	destination[0], destination[1] = byte(value), byte(value>>8)
	destination[2], destination[3] = byte(value>>16), byte(value>>24)
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event != playdate.LifecycleTerminate {
		return nil
	}
	g.closed = true
	var recorderErr, playbackErr error
	if g.recorder != nil {
		recorderErr = g.recorder.Close()
		g.recorder = nil
	}
	if g.playback != nil {
		playbackErr = g.playback.Close()
		g.playback = nil
	}
	return errors.Join(recorderErr, playbackErr)
}

func permissionName(permission playdate.MicrophonePermission) string {
	switch permission {
	case playdate.MicrophonePermissionGranted:
		return "granted"
	case playdate.MicrophonePermissionDenied:
		return "denied"
	default:
		return "pending"
	}
}
