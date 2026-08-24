// Package navigation exercises the P3 application-menu and Launcher-exit contract.
package navigation

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

var errLauncherUnavailable = errors.New("Playdate Launcher capability is unavailable")

type phase uint8

const (
	phaseMenu phase = iota
	phasePlaying
)

type game struct {
	phase    phase
	selected int
}

// New creates the P3 application-navigation acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(playdate.Context) error {
	g.phase = phaseMenu
	g.selected = 0
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	input := context.Input()
	if g.phase == phasePlaying {
		if input.Pressed.Has(playdate.ButtonB) {
			g.phase = phaseMenu
		}
		g.draw(context)
		return true, nil
	}

	if input.Pressed.Has(playdate.ButtonUp) || input.Pressed.Has(playdate.ButtonDown) {
		g.selected = 1 - g.selected
	}
	if input.Pressed.Has(playdate.ButtonA) {
		if g.selected == 0 {
			g.phase = phasePlaying
		} else {
			launcher, ok := context.(playdate.Launcher)
			if !ok {
				return false, errLauncherUnavailable
			}
			launcher.ExitToLauncher()
		}
	}
	g.draw(context)
	return true, nil
}

func (g *game) draw(context playdate.Context) {
	context.Clear()
	if g.phase == phasePlaying {
		context.DrawText("PLAYING", 166, 94)
		context.DrawText("B: MAIN MENU", 138, 126)
		return
	}
	context.DrawText("P3 APPLICATION MENU", 116, 52)
	prefixes := [2]string{"  ", "  "}
	prefixes[g.selected] = "> "
	context.DrawText(prefixes[0]+"PLAY", 166, 96)
	context.DrawText(prefixes[1]+"EXIT", 166, 124)
	context.DrawText("UP/DOWN + A", 150, 168)
}
