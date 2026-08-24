package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type statusContext struct {
	testContext
	accelerometerCalls []bool
	elapsedReset       bool
	exited             bool
}

func (context *statusContext) SetAccelerometerEnabled(enabled bool) {
	context.accelerometerCalls = append(context.accelerometerCalls, enabled)
}
func (*statusContext) AccelerometerXYZ() (float32, float32, float32) { return 1, -2, 3 }
func (*statusContext) PowerStatus() playdate.PowerStatus {
	return playdate.PowerCharging | playdate.PowerUSB
}
func (*statusContext) BatteryPercentage() float32   { return 72.5 }
func (*statusContext) BatteryVoltage() float32      { return 4.1 }
func (*statusContext) SystemVolume() float32        { return 0.6 }
func (*statusContext) ReduceFlashing() bool         { return true }
func (*statusContext) TimezoneOffsetSeconds() int32 { return 7200 }
func (*statusContext) Uses24HourTime() bool         { return true }
func (*statusContext) CurrentEpochTime() playdate.EpochTime {
	return playdate.EpochTime{Seconds: 123456, Milliseconds: 789}
}
func (*statusContext) EpochToDateTime(epoch uint32) playdate.DateTime {
	return playdate.DateTime{Year: 2000, Month: 1, Day: uint8(epoch + 1), Weekday: 6}
}
func (*statusContext) DateTimeToEpoch(value playdate.DateTime) (uint32, error) {
	return uint32(value.Day - 1), nil
}
func (context *statusContext) ResetElapsedTime() { context.elapsedReset = true }
func (*statusContext) ElapsedTime() float32      { return 0.125 }
func (*statusContext) SystemInfo() playdate.SystemInfo {
	return playdate.SystemInfo{OSVersion: 30101, Language: playdate.LanguageJapanese, PDXVersion: 30100}
}
func (context *statusContext) ExitToLauncher() { context.exited = true }

type statusGame struct{}

func (statusGame) Init(context playdate.Context) error {
	accelerometer, ok := context.(playdate.Accelerometer)
	if !ok {
		return errors.New("accelerometer capability missing")
	}
	if x, y, z := accelerometer.AccelerometerXYZ(); x != 0 || y != 0 || z != 0 {
		return errors.New("accelerometer sampled before enablement")
	}
	accelerometer.SetAccelerometerEnabled(true)
	if x, y, z := accelerometer.AccelerometerXYZ(); x != 1 || y != -2 || z != 3 {
		return errors.New("accelerometer values not forwarded")
	}
	monitor, ok := context.(playdate.PowerMonitor)
	if !ok || !monitor.PowerStatus().Has(playdate.PowerCharging|playdate.PowerUSB) || monitor.BatteryPercentage() != 72.5 || monitor.BatteryVoltage() != 4.1 {
		return errors.New("power values not forwarded")
	}
	preferences, ok := context.(playdate.SystemPreferences)
	if !ok || preferences.SystemVolume() != 0.6 || !preferences.ReduceFlashing() || preferences.TimezoneOffsetSeconds() != 7200 || !preferences.Uses24HourTime() {
		return errors.New("preferences not forwarded")
	}
	environment, ok := context.(playdate.SystemEnvironment)
	if !ok {
		return errors.New("system environment capability missing")
	}
	if now := environment.CurrentEpochTime(); now != (playdate.EpochTime{Seconds: 123456, Milliseconds: 789}) {
		return errors.New("current epoch time not forwarded")
	}
	date := environment.EpochToDateTime(0)
	if date.Year != 2000 || date.Month != 1 || date.Day != 1 || date.Weekday != 6 {
		return errors.New("epoch conversion not forwarded")
	}
	if epoch, err := environment.DateTimeToEpoch(date); err != nil || epoch != 0 {
		return errors.New("date conversion not forwarded")
	}
	environment.ResetElapsedTime()
	if environment.ElapsedTime() != 0.125 {
		return errors.New("elapsed time not forwarded")
	}
	if info := environment.SystemInfo(); info.OSVersion != 30101 || info.Language != playdate.LanguageJapanese || info.PDXVersion != 30100 {
		return errors.New("system information not forwarded")
	}
	launcher, ok := context.(playdate.Launcher)
	if !ok {
		return errors.New("launcher capability missing")
	}
	launcher.ExitToLauncher()
	return nil
}
func (statusGame) Update(playdate.Context) (bool, error) { return false, nil }

func TestNewApplicationForwardsStatusAndCleansUpAccelerometer(t *testing.T) {
	context := &statusContext{}
	application, err := NewApplication(statusGame{}, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if !context.exited {
		t.Fatal("ExitToLauncher was not forwarded")
	}
	if !context.elapsedReset {
		t.Fatal("ResetElapsedTime was not forwarded")
	}
	if err := application.Handle(EventTerminate, 0); err != nil {
		t.Fatal(err)
	}
	if len(context.accelerometerCalls) != 2 || !context.accelerometerCalls[0] || context.accelerometerCalls[1] {
		t.Fatalf("accelerometer enablement calls = %v, want [true false]", context.accelerometerCalls)
	}
}

func TestValidateDateTime(t *testing.T) {
	valid := []playdate.DateTime{
		{Year: 2000, Month: 1, Day: 1},
		{Year: 2000, Month: 2, Day: 29, Hour: 23, Minute: 59, Second: 59},
		{Year: 2136, Month: 2, Day: 7, Hour: 6, Minute: 28, Second: 15},
	}
	for _, value := range valid {
		if err := ValidateDateTime(value); err != nil {
			t.Errorf("ValidateDateTime(%+v) = %v", value, err)
		}
	}
	invalid := []playdate.DateTime{
		{Year: 1999, Month: 12, Day: 31},
		{Year: 2001, Month: 2, Day: 29},
		{Year: 2000, Month: 13, Day: 1},
		{Year: 2000, Month: 1, Day: 1, Hour: 24},
		{Year: 2136, Month: 2, Day: 7, Hour: 6, Minute: 28, Second: 16},
	}
	for _, value := range invalid {
		if err := ValidateDateTime(value); !errors.Is(err, playdate.ErrDateTime) {
			t.Errorf("ValidateDateTime(%+v) = %v, want ErrDateTime", value, err)
		}
	}
}
