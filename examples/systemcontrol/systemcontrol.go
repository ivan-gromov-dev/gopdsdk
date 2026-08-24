// Package systemcontrol exercises P10.1 launch, menu-image, system-setting,
// mirror-lifecycle, and lossless button-event behavior.
package systemcontrol

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const buttonQueueSize = 16

type game struct {
	controls          playdate.SystemControls
	menuImage         playdate.Bitmap
	launchArguments   string
	launchPath        string
	mirrorState       string
	lastButton        playdate.ButtonEvent
	buttonTransitions uint32
	restartRequested  bool
	autoLockDisabled  bool
	crankMuted        bool
}

// New creates the P10.1 acceptance scene.
func New() playdate.Game { return &game{mirrorState: "idle"} }

func (game *game) Init(context playdate.Context) error {
	controls, ok := any(context).(playdate.SystemControls)
	if !ok {
		return playdate.ErrSystemControlsUnavailable
	}
	game.controls = controls
	game.launchArguments, game.launchPath = controls.LaunchArguments()

	image, err := context.NewBitmap(400, 240)
	if err != nil {
		return err
	}
	game.menuImage = image
	if err := image.Fill(playdate.ColorWhite); err != nil {
		return err
	}
	if offscreen, ok := any(context).(playdate.OffscreenGraphics); ok {
		if err := offscreen.DrawInto(image, func() error {
			context.DrawText("gopdsdk P10.1", 16, 32)
			context.DrawText("launch + lifecycle", 16, 56)
			return nil
		}); err != nil {
			return err
		}
	}
	if err := controls.SetMenuImage(image, 24); err != nil {
		return err
	}
	game.autoLockDisabled = true
	controls.SetAutoLockDisabled(true)
	game.crankMuted = true
	controls.SetCrankSoundsDisabled(true)
	return controls.SetButtonCallback(game.handleButton, buttonQueueSize)
}

func (game *game) handleButton(event playdate.ButtonEvent) {
	game.lastButton = event
	game.buttonTransitions++
	if event.Button == playdate.ButtonA && event.Down {
		game.restartRequested = true
	}
}

func (game *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	switch event {
	case playdate.LifecycleMirrorStarted:
		game.mirrorState = "started"
	case playdate.LifecycleMirrorEnded:
		game.mirrorState = "ended"
	case playdate.LifecycleTerminate:
		if game.menuImage != nil {
			return game.menuImage.Close()
		}
	}
	return nil
}

func (game *game) Update(context playdate.Context) (bool, error) {
	if game.restartRequested {
		game.restartRequested = false
		if err := game.controls.RestartGame("p10-restarted"); err != nil {
			return false, err
		}
	}
	if context.Input().Pressed.Has(playdate.ButtonB) {
		game.autoLockDisabled = !game.autoLockDisabled
		game.crankMuted = !game.crankMuted
		game.controls.SetAutoLockDisabled(game.autoLockDisabled)
		game.controls.SetCrankSoundsDisabled(game.crankMuted)
	}

	context.Clear()
	context.DrawText("P10.1 SYSTEM CONTROL", 12, 8)
	context.DrawText("ARGS: "+emptyText(game.launchArguments), 12, 36)
	context.DrawText("PATH: "+emptyText(game.launchPath), 12, 60)
	context.DrawText("MIRROR: "+game.mirrorState, 12, 84)
	context.DrawText(buttonLine(game.lastButton, game.buttonTransitions), 12, 108)
	context.DrawText("OVERFLOW: "+count(game.controls.ButtonCallbackOverflow()), 12, 132)
	context.DrawText("AUTOLOCK OFF: "+strconv.FormatBool(game.autoLockDisabled), 12, 156)
	context.DrawText("CRANK MUTED: "+strconv.FormatBool(game.crankMuted), 12, 180)
	context.DrawText("A: RESTART  B: TOGGLE", 12, 212)
	return true, nil
}

func emptyText(value string) string {
	if value == "" {
		return "<empty>"
	}
	return value
}

func buttonLine(event playdate.ButtonEvent, transitions uint32) string {
	state := "up"
	if event.Down {
		state = "down"
	}
	return "BUTTON: " + buttonBits(event.Button) + " " + state + " @" + count(event.When) + " #" + count(transitions)
}

func buttonBits(buttons playdate.Buttons) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[buttons>>4], digits[buttons&0x0f]})
}

func count(value uint32) string { return strconv.FormatUint(uint64(value), 10) }
