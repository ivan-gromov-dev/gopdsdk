package compliant

import (
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	pdjson "github.com/ivan-gromov-dev/gopdsdk/playdate/json"
	"github.com/ivan-gromov-dev/gopdsdk/playdate/schedule"
)

// analyzer-contract: device-scheduler-replacement positive
func boundedWork(scheduler *schedule.Scheduler) error {
	_, err := scheduler.Schedule(schedule.Complete)
	return err
}

// analyzer-contract: device-json-replacement positive
func boundedJSON(data []byte) error {
	_, err := pdjson.DecodeBytes(data, pdjson.Limits{})
	return err
}

// analyzer-contract: device-fmt-replacement positive
func boundedInteger(destination []byte, value int64) []byte {
	return strconv.AppendInt(destination, value, 10)
}

// analyzer-contract: device-panic-profile positive
func normalReturnCleanup(close func()) { defer close() }

// analyzer-contract: device-cgo-profile positive
func publicGoSurface(context playdate.Context) uint32 { return context.CurrentTimeMilliseconds() }

// analyzer-contract: device-reflection-profile positive
func reflectedKind(value any) reflect.Kind { return reflect.TypeOf(value).Kind() }

// analyzer-contract: device-runtime-control-profile positive
func explicitGC() { runtime.GC() }

func durationOperations(text string, duration time.Duration) (time.Duration, string) {
	parsed, _ := time.ParseDuration(text)
	return parsed.Round(time.Millisecond), duration.String()
}

// analyzer-contract: context-optional-capability positive
// analyzer-contract: video-capability-availability positive
func optionalCapabilities(context playdate.Context) {
	if launcher, ok := context.(playdate.Launcher); ok {
		launcher.ExitToLauncher()
	}
	if videos, ok := context.(playdate.Videos); ok {
		_, _ = videos.LoadVideo("intro.pdv")
	}
}

// analyzer-contract: framebuffer-callback-scope positive
func copyFramebuffer(graphics playdate.FramebufferGraphics) ([]byte, error) {
	var copied []byte
	err := graphics.WithFramebuffer(func(frame playdate.Framebuffer) error {
		bytes, err := frame.Bytes()
		if err == nil {
			copied = append(copied, bytes...)
		}
		return err
	})
	return copied, err
}

// analyzer-contract: bitmap-data-callback-scope positive
func copyBitmapData(graphics playdate.BitmapDataGraphics, bitmap playdate.Bitmap) ([]byte, error) {
	var copied []byte
	err := graphics.WithBitmapData(bitmap, func(data playdate.BitmapData) error {
		bytes, err := data.Bytes()
		if err == nil {
			copied = append(copied, bytes...)
		}
		return err
	})
	return copied, err
}

// analyzer-contract: microphone-samples-callback-scope positive
func copyMicrophoneSamples(samples playdate.MicrophoneSamples, destination []int16) error {
	_, err := samples.CopyTo(destination)
	return err
}

// analyzer-contract: audio-render-buffer-callback-scope positive
func renderSilence(left, right []int16) int {
	clear(left)
	clear(right)
	return len(left)
}

// analyzer-contract: bitmap-owned-handle positive
// analyzer-contract: bitmap-close-semantics positive
func useOwnedBitmap(graphics playdate.Graphics) (err error) {
	bitmap, err := graphics.LoadBitmap("hero")
	if err != nil {
		return err
	}
	defer func() { err = bitmap.Close() }()
	return graphics.DrawBitmap(bitmap, 0, 0)
}

// analyzer-contract: bitmap-table-borrowed-frame positive
func useBorrowedFrame(table playdate.BitmapTable) error {
	frame, err := table.Frame(0)
	if err != nil {
		return err
	}
	_, err = frame.Width()
	return err
}

// analyzer-contract: bitmap-table-close-semantics positive
func closeBitmapTable(table playdate.BitmapTable) error { return table.Close() }

// analyzer-contract: sprite-close-semantics positive
func closeSprite(sprite playdate.Sprite) error {
	if err := sprite.Remove(); err != nil {
		return err
	}
	return sprite.Close()
}

// analyzer-contract: audio-close-semantics positive
func closeSoundEffect(effect playdate.SoundEffect) error { return effect.Close() }

// analyzer-contract: font-close-semantics positive
func closeFont(font playdate.Font) error { return font.Close() }

// analyzer-contract: file-close-semantics positive
func closeFile(file playdate.File) error { return file.Close() }

// analyzer-contract: video-close-semantics positive
func closeVideo(player playdate.VideoPlayer) error { return player.Close() }

// analyzer-contract: menu-image-retention positive
// analyzer-contract: menu-image-offset-bound positive
func installMenuImage(controls playdate.SystemControls, bitmap playdate.Bitmap) error {
	if err := controls.SetMenuImage(bitmap, 200); err != nil {
		return err
	}
	controls.ClearMenuImage()
	return bitmap.Close()
}

type game struct{ scheduler *schedule.Scheduler }

func (*game) Init(playdate.Context) error { return nil }

// analyzer-contract: scheduler-update-boundary positive
func (game *game) Update(playdate.Context) (bool, error) {
	_, err := game.scheduler.Update()
	return true, err
}
