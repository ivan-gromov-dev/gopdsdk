package systemstatus

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestPowerText(t *testing.T) {
	for _, test := range []struct {
		status playdate.PowerStatus
		want   string
	}{
		{0, "NONE"},
		{playdate.PowerUSB, "USB"},
		{playdate.PowerCharging | playdate.PowerUSB, "CHARGE+USB"},
		{playdate.PowerScrews, "SCREWS"},
	} {
		if got := powerText(test.status); got != test.want {
			t.Errorf("powerText(%d) = %q, want %q", test.status, got, test.want)
		}
	}
}
