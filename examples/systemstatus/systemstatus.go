// Package systemstatus exercises the P4.4 motion, power, preferences, and launcher contract.
package systemstatus

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type game struct {
	accelerometer playdate.Accelerometer
	power         playdate.PowerMonitor
	preferences   playdate.SystemPreferences
	launcher      playdate.Launcher
	result        string
}

// New creates the P4.4 acceptance scene.
func New() playdate.Game { return &game{} }

func (game *game) Init(context playdate.Context) error {
	var ok bool
	game.accelerometer, ok = any(context).(playdate.Accelerometer)
	if !ok {
		game.result = "FAIL: ACCELEROMETER"
		return nil
	}
	game.power, ok = any(context).(playdate.PowerMonitor)
	if !ok {
		game.result = "FAIL: POWER"
		return nil
	}
	game.preferences, ok = any(context).(playdate.SystemPreferences)
	if !ok {
		game.result = "FAIL: PREFERENCES"
		return nil
	}
	game.launcher, ok = any(context).(playdate.Launcher)
	if !ok {
		game.result = "FAIL: LAUNCHER"
		return nil
	}
	game.accelerometer.SetAccelerometerEnabled(true)
	game.result = "P4.4 STATUS OK"
	return nil
}

func (game *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText(game.result, 12, 12)
	if game.accelerometer == nil {
		return true, nil
	}
	x, y, z := game.accelerometer.AccelerometerXYZ()
	context.DrawText("ACC "+decimal(x)+" "+decimal(y)+" "+decimal(z), 12, 42)
	context.DrawText("BAT "+decimal(game.power.BatteryPercentage())+"% "+decimal(game.power.BatteryVoltage())+"V", 12, 72)
	context.DrawText("POWER "+powerText(game.power.PowerStatus()), 12, 102)
	context.DrawText("VOL "+decimal(game.preferences.SystemVolume())+" FLASH "+strconv.FormatBool(game.preferences.ReduceFlashing()), 12, 132)
	context.DrawText("TZ "+strconv.FormatInt(int64(game.preferences.TimezoneOffsetSeconds()), 10)+" 24H "+strconv.FormatBool(game.preferences.Uses24HourTime()), 12, 162)
	context.DrawText("B: EXIT TO LAUNCHER", 12, 202)
	if context.Input().Pressed.Has(playdate.ButtonB) {
		game.launcher.ExitToLauncher()
	}
	return true, nil
}

func decimal(value float32) string { return strconv.FormatFloat(float64(value), 'f', 2, 32) }

func powerText(status playdate.PowerStatus) string {
	value := ""
	for _, flag := range []struct {
		value playdate.PowerStatus
		label string
	}{{playdate.PowerCharging, "CHARGE"}, {playdate.PowerUSB, "USB"}, {playdate.PowerScrews, "SCREWS"}} {
		if status.Has(flag.value) {
			if value != "" {
				value += "+"
			}
			value += flag.label
		}
	}
	if value == "" {
		return "NONE"
	}
	return value
}
