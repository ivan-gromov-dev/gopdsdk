package microphone

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type files struct{ directory string }
type testFile struct{ *os.File }

func (file testFile) Flush() error { return file.Sync() }

func (files files) OpenFile(path string, options playdate.FileOptions) (playdate.File, error) {
	if options != playdate.FileWrite {
		panic(options)
	}
	file, err := os.Create(filepath.Join(files.directory, path))
	return testFile{file}, err
}
func (files) Stat(string) (playdate.FileInfo, error) { return playdate.FileInfo{}, nil }
func (files) List(string, bool) ([]string, error)    { return nil, nil }
func (files) Mkdir(string) error                     { return nil }
func (files) Remove(string, bool) error              { return nil }
func (files) Rename(string, string) error            { return nil }

type samples struct{ values []int16 }

func (samples samples) Len() int { return len(samples.values) }
func (samples samples) CopyTo(destination []int16) (int, error) {
	return copy(destination, samples.values), nil
}

type recorder struct{ closed bool }

func (*recorder) Source() playdate.MicrophoneSource { return playdate.MicrophoneSourceInternal }
func (recorder *recorder) Stop() error              { recorder.closed = true; return nil }
func (recorder *recorder) Close() error             { recorder.closed = true; return nil }

type failedPlayer struct{ recorder }

func (*failedPlayer) Play() error                            { return playdate.ErrAudioPlay }
func (*failedPlayer) Pause() error                           { return nil }
func (*failedPlayer) Resume() error                          { return nil }
func (*failedPlayer) SetVolume(float32, float32) error       { return nil }
func (*failedPlayer) Volume() (float32, float32, error)      { return 1, 1, nil }
func (*failedPlayer) State() (playdate.PlaybackState, error) { return playdate.PlaybackStopped, nil }
func (*failedPlayer) PlayRepeated(int, float32) error        { return playdate.ErrAudioPlay }
func (*failedPlayer) Length() (float32, error)               { return 1, nil }
func (*failedPlayer) SetOffset(float32) error                { return nil }
func (*failedPlayer) Offset() (float32, error)               { return 0, nil }
func (*failedPlayer) SetRate(float32) error                  { return nil }
func (*failedPlayer) Rate() (float32, error)                 { return 1, nil }

type pcmPlayers struct{ player *failedPlayer }

func (players pcmPlayers) NewPCMPlayer([]int16, uint32) (playdate.SamplePlayer, error) {
	return players.player, nil
}

func TestSamplesAreBoundedAndLifecycleClosesRecorder(t *testing.T) {
	g := &game{}
	values := make([]int16, 300)
	values[0], values[1] = -123, 456
	if !g.samples(samples{values: values}) {
		t.Fatal("samples stopped an open game")
	}
	if g.peak != 456 || g.blocks != 1 {
		t.Fatalf("peak/blocks = %d/%d", g.peak, g.blocks)
	}
	native := &recorder{}
	g.recorder = native
	if err := g.HandleLifecycle(nil, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if !native.closed || g.recorder != nil {
		t.Fatal("termination did not release recorder")
	}
}

func TestWriteWAVUsesRecordedBound(t *testing.T) {
	directory := t.TempDir()
	g := &game{files: files{directory: directory}, recorded: 3}
	g.capture[0], g.capture[1], g.capture[2], g.capture[3] = -1, 2, -3, 30000
	if err := g.writeWAV(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, recordingFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" || len(data) != 50 {
		t.Fatalf("invalid WAV header/length: %q %d", data[:12], len(data))
	}
	if got := binary.LittleEndian.Uint32(data[40:44]); got != 6 {
		t.Fatalf("data size = %d", got)
	}
	if got := int16(binary.LittleEndian.Uint16(data[48:50])); got != -3 {
		t.Fatalf("last sample = %d", got)
	}
}

func TestPlaybackFailureDoesNotRetainClosedPlayer(t *testing.T) {
	player := &failedPlayer{}
	g := &game{pcmPlayers: pcmPlayers{player: player}, recorded: 1}
	if err := g.startPlayback(); err == nil {
		t.Fatal("startPlayback succeeded")
	}
	if g.playback != nil || !player.closed {
		t.Fatal("failed playback retained an owned player")
	}
}
