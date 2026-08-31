package noncompliant

import (
	"encoding/json"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	"github.com/ivan-gromov-dev/gopdsdk/playdate/schedule"
)

var escapedFramebuffer []byte

// analyzer-contract: device-scheduler-replacement negative
func unboundedWork() {
	go func() {}()
}

// analyzer-contract: device-json-replacement negative
func reflectionJSON(data []byte) error {
	return json.Unmarshal(data, new(any))
}

// analyzer-contract: context-optional-capability negative
// analyzer-contract: video-capability-availability negative
func assumedCapabilities(context playdate.Context) {
	context.(playdate.Launcher).ExitToLauncher()
	_, _ = context.(playdate.Videos).LoadVideo("intro.pdv")
}

// analyzer-contract: framebuffer-callback-scope negative
func retainFramebuffer(graphics playdate.FramebufferGraphics) error {
	return graphics.WithFramebuffer(func(frame playdate.Framebuffer) error {
		var err error
		escapedFramebuffer, err = frame.Bytes()
		return err
	})
}

// analyzer-contract: bitmap-owned-handle negative
func leakBitmap(graphics playdate.Graphics) error {
	_, err := graphics.LoadBitmap("hero")
	return err
}

// analyzer-contract: bitmap-table-borrowed-frame negative
func closeBorrowedFrame(table playdate.BitmapTable) error {
	frame, err := table.Frame(0)
	if err != nil {
		return err
	}
	return frame.Close()
}

// analyzer-contract: bitmap-table-close-semantics negative
func useFrameAfterTableClose(table playdate.BitmapTable) error {
	frame, err := table.Frame(0)
	if err != nil {
		return err
	}
	if err := table.Close(); err != nil {
		return err
	}
	_, err = frame.Width()
	return err
}

// analyzer-contract: sprite-close-semantics negative
func closeSpriteTwice(sprite playdate.Sprite) error {
	_ = sprite.Close()
	return sprite.Close()
}

// analyzer-contract: audio-close-semantics negative
func closeSoundEffectTwice(effect playdate.SoundEffect) error {
	_ = effect.Close()
	return effect.Close()
}

// analyzer-contract: font-close-semantics negative
func closeFontTwice(font playdate.Font) error {
	_ = font.Close()
	return font.Close()
}

// analyzer-contract: file-close-semantics negative
func discardFileCloseError(file playdate.File) { _ = file.Close() }

// analyzer-contract: video-close-semantics negative
func closeVideoTwice(player playdate.VideoPlayer) error {
	_ = player.Close()
	return player.Close()
}

// analyzer-contract: menu-image-retention negative
func closeRetainedMenuImage(controls playdate.SystemControls, bitmap playdate.Bitmap) error {
	if err := controls.SetMenuImage(bitmap, 0); err != nil {
		return err
	}
	return bitmap.Close()
}

// analyzer-contract: bitmap-close-semantics negative
func useClosedBitmap(graphics playdate.Graphics, bitmap playdate.Bitmap) error {
	if err := bitmap.Close(); err != nil {
		return err
	}
	return graphics.DrawBitmap(bitmap, 0, 0)
}

// analyzer-contract: scheduler-update-boundary negative
func initializeScheduler(scheduler *schedule.Scheduler) error {
	_, err := scheduler.Update()
	return err
}

// analyzer-contract: menu-image-offset-bound negative
func invalidMenuOffset(controls playdate.SystemControls, bitmap playdate.Bitmap) error {
	return controls.SetMenuImage(bitmap, 201)
}
