// Package systemenvironment exercises the P10.2 offline clock, calendar, and
// copied device-information contract.
package systemenvironment

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type game struct {
	environment playdate.SystemEnvironment
}

// New creates the P10.2 acceptance scene.
func New() playdate.Game { return &game{} }

func (game *game) Init(context playdate.Context) error {
	environment, ok := any(context).(playdate.SystemEnvironment)
	if !ok {
		return playdate.ErrSystemEnvironmentUnavailable
	}
	game.environment = environment
	environment.ResetElapsedTime()
	return nil
}

func (game *game) Update(context playdate.Context) (bool, error) {
	if context.Input().Pressed.Has(playdate.ButtonA) {
		game.environment.ResetElapsedTime()
	}
	now := game.environment.CurrentEpochTime()
	date := game.environment.EpochToDateTime(now.Seconds)
	roundTrip, err := game.environment.DateTimeToEpoch(date)
	if err != nil {
		return false, err
	}
	info := game.environment.SystemInfo()
	context.Clear()
	context.DrawText("P10.2 SYSTEM ENVIRONMENT", 12, 12)
	context.DrawText("EPOCH "+number(now.Seconds)+"."+milliseconds(now.Milliseconds), 12, 44)
	context.DrawText(dateText(date), 12, 76)
	context.DrawText("ROUNDTRIP "+number(roundTrip), 12, 108)
	context.DrawText("ELAPSED "+decimal(game.environment.ElapsedTime()), 12, 140)
	context.DrawText("OS "+number(info.OSVersion)+" PDX "+number(info.PDXVersion), 12, 172)
	context.DrawText("LANG "+strconv.Itoa(int(info.Language))+"  A: RESET", 12, 204)
	return true, nil
}

func number(value uint32) string { return strconv.FormatUint(uint64(value), 10) }
func decimal(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', 3, 32)
}
func milliseconds(value uint32) string {
	text := number(value)
	for len(text) < 3 {
		text = "0" + text
	}
	return text
}
func dateText(value playdate.DateTime) string {
	return number(uint32(value.Year)) + "-" + two(value.Month) + "-" + two(value.Day) +
		" " + two(value.Hour) + ":" + two(value.Minute) + ":" + two(value.Second) +
		" W" + number(uint32(value.Weekday))
}
func two(value uint8) string {
	if value < 10 {
		return "0" + number(uint32(value))
	}
	return number(uint32(value))
}
