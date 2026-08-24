package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestMicrophonePermissionAndRecordingLifetime(t *testing.T) {
	var permissionCallback func(bool)
	var nativeCallback func([]int16) bool
	stops := 0
	service := NewMicrophoneService(MicrophoneDriver{
		Request: func(purpose string, callback func(bool)) playdate.MicrophonePermission {
			if purpose != "voice commands" {
				t.Fatalf("purpose = %q", purpose)
			}
			permissionCallback = callback
			return playdate.MicrophonePermissionPending
		},
		Start: func(source playdate.MicrophoneSource, callback func([]int16) bool) playdate.MicrophoneSource {
			if source != playdate.MicrophoneSourceAutomatic {
				t.Fatalf("source = %v", source)
			}
			nativeCallback = callback
			return playdate.MicrophoneSourceInternal
		},
		Stop: func() { stops++ },
	})

	var permission playdate.MicrophonePermission
	got, err := service.RequestMicrophoneAccess("voice commands", func(value playdate.MicrophonePermission) { permission = value })
	if err != nil || got != playdate.MicrophonePermissionPending {
		t.Fatalf("request = %v, %v", got, err)
	}
	permissionCallback(true)
	if permission != playdate.MicrophonePermissionGranted {
		t.Fatalf("permission = %v", permission)
	}

	buffer := make([]int16, 2)
	var retained playdate.MicrophoneSamples
	recorder, err := service.StartMicrophoneRecording(playdate.MicrophoneSourceAutomatic, func(samples playdate.MicrophoneSamples) bool {
		retained = samples
		n, copyErr := samples.CopyTo(buffer)
		if copyErr != nil || n != len(buffer) {
			t.Fatalf("copy = %d, %v", n, copyErr)
		}
		return true
	})
	if err != nil || recorder.Source() != playdate.MicrophoneSourceInternal {
		t.Fatalf("start = %v, %v", recorder, err)
	}
	if !nativeCallback([]int16{11, 22, 33}) {
		t.Fatal("recording stopped")
	}
	if buffer[0] != 11 || buffer[1] != 22 {
		t.Fatalf("buffer = %v", buffer)
	}
	if _, err := retained.CopyTo(buffer); !errors.Is(err, playdate.ErrMicrophoneSamplesExpired) {
		t.Fatalf("expired error = %v", err)
	}
	if err := recorder.Close(); err != nil || stops != 1 {
		t.Fatalf("close = %v, stops %d", err, stops)
	}
}

func TestMicrophoneReplacementDenialAndOverflow(t *testing.T) {
	var permissionCallback func(bool)
	var callbacks []func([]int16) bool
	stops := 0
	service := NewMicrophoneService(MicrophoneDriver{
		Request: func(_ string, callback func(bool)) playdate.MicrophonePermission {
			permissionCallback = callback
			return playdate.MicrophonePermissionPending
		},
		Start: func(_ playdate.MicrophoneSource, callback func([]int16) bool) playdate.MicrophoneSource {
			callbacks = append(callbacks, callback)
			return playdate.MicrophoneSourceHeadset
		},
		Stop: func() { stops++ },
	})
	_, _ = service.RequestMicrophoneAccess("", func(playdate.MicrophonePermission) {})
	first, _ := service.StartMicrophoneRecording(playdate.MicrophoneSourceHeadset, func(playdate.MicrophoneSamples) bool { return true })
	second, _ := service.StartMicrophoneRecording(playdate.MicrophoneSourceAutomatic, func(samples playdate.MicrophoneSamples) bool {
		destination := make([]int16, 1)
		n, err := samples.CopyTo(destination)
		return err == nil && n == 1
	})
	if stops != 1 {
		t.Fatalf("replacement stops = %d", stops)
	}
	if err := first.Stop(); !errors.Is(err, playdate.ErrMicrophoneClosed) {
		t.Fatalf("old recorder error = %v", err)
	}
	if callbacks[0]([]int16{1}) {
		t.Fatal("replaced callback continued")
	}
	if !callbacks[1]([]int16{1, 2, 3}) {
		t.Fatal("bounded copy stopped recording")
	}
	permissionCallback(false)
	if stops != 2 {
		t.Fatalf("denial stops = %d", stops)
	}
	if err := second.Stop(); !errors.Is(err, playdate.ErrMicrophoneClosed) {
		t.Fatalf("denied recorder error = %v", err)
	}
}

type microphoneContext struct {
	*MicrophoneService
	cleaned *bool
}

func (context microphoneContext) CloseMicrophone() {
	context.MicrophoneService.Close()
	if context.cleaned != nil {
		*context.cleaned = true
	}
}

func (microphoneContext) CurrentTimeMilliseconds() uint32                      { return 0 }
func (microphoneContext) Clear()                                               {}
func (microphoneContext) DrawText(string, int, int)                            {}
func (microphoneContext) LoadBitmap(string) (playdate.Bitmap, error)           { return nil, nil }
func (microphoneContext) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (microphoneContext) NewBitmap(int, int) (playdate.Bitmap, error)          { return nil, nil }
func (microphoneContext) DrawBitmap(playdate.Bitmap, int, int) error           { return nil }
func (microphoneContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error {
	return nil
}
func (microphoneContext) Input() playdate.Input                                  { return playdate.Input{} }
func (microphoneContext) NewSprite() (playdate.Sprite, error)                    { return nil, nil }
func (microphoneContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite { return nil }
func (microphoneContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite     { return nil }
func (microphoneContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (microphoneContext) UpdateAndDrawSprites()                                {}
func (microphoneContext) LoadSoundEffect(string) (playdate.SoundEffect, error) { return nil, nil }
func (microphoneContext) LoadFilePlayer(string) (playdate.FilePlayer, error)   { return nil, nil }
func (microphoneContext) NewPCMPlayer(samples []int16, rate uint32) (playdate.SamplePlayer, error) {
	if len(samples) != 2 || rate != 44100 {
		return nil, errors.New("unexpected PCM input")
	}
	return NewSamplePlayer(9, AudioDriver{}), nil
}

func TestNewApplicationForwardsAndStopsMicrophone(t *testing.T) {
	stops := 0
	cleaned := false
	service := NewMicrophoneService(MicrophoneDriver{Start: func(playdate.MicrophoneSource, func([]int16) bool) playdate.MicrophoneSource {
		return playdate.MicrophoneSourceInternal
	}, Stop: func() { stops++ }})
	game := testGame{init: func(context playdate.Context) error {
		microphones, ok := context.(playdate.Microphones)
		if !ok {
			t.Fatal("Microphones capability missing")
		}
		_, err := microphones.StartMicrophoneRecording(playdate.MicrophoneSourceInternal, func(playdate.MicrophoneSamples) bool { return true })
		if err != nil {
			return err
		}
		_, err = context.(playdate.PCMPlayers).NewPCMPlayer([]int16{1, 2}, 44100)
		return err
	}}
	application, err := NewApplication(game, microphoneContext{MicrophoneService: service, cleaned: &cleaned}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventTerminate, 0); err != nil {
		t.Fatal(err)
	}
	if stops != 1 {
		t.Fatalf("termination stops = %d", stops)
	}
	if !cleaned {
		t.Fatal("termination did not release microphone callbacks")
	}
}
