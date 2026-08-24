// Package lifecycleinput displays the portable lifecycle and input model for
// Simulator and physical-device parity acceptance.
package lifecycleinput

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const requiredSoakSeconds = 60

type game struct {
	lifecycle      string
	elapsedSeconds float32
	soakComplete   bool
	lastPressed    playdate.Buttons
	lastReleased   playdate.Buttons
	lifecycleTrace [4]string
	traceLength    int
	pauseCount     uint32
	resumeCount    uint32
	lockCount      uint32
	unlockCount    uint32
	lowPowerCount  uint32
}

// New creates the lifecycle and input parity probe game.
func New() playdate.Game { return &game{lifecycle: "init"} }

func (g *game) Init(playdate.Context) error { return nil }

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	g.lifecycle = lifecycleName(event)
	g.recordLifecycle(g.lifecycle)
	switch event {
	case playdate.LifecyclePause:
		g.pauseCount++
	case playdate.LifecycleResume:
		g.resumeCount++
	case playdate.LifecycleLock:
		g.lockCount++
	case playdate.LifecycleUnlock:
		g.unlockCount++
	case playdate.LifecycleLowPower:
		g.lowPowerCount++
	}
	return nil
}

func (g *game) recordLifecycle(event string) {
	if g.traceLength < len(g.lifecycleTrace) {
		g.lifecycleTrace[g.traceLength] = event
		g.traceLength++
		return
	}
	copy(g.lifecycleTrace[:], g.lifecycleTrace[1:])
	g.lifecycleTrace[len(g.lifecycleTrace)-1] = event
}

func (g *game) Update(context playdate.Context) (bool, error) {
	input := context.Input()
	g.elapsedSeconds += input.DeltaSeconds
	if g.elapsedSeconds >= requiredSoakSeconds {
		g.soakComplete = true
	}
	if input.Pressed != 0 {
		g.lastPressed = input.Pressed
	}
	if input.Released != 0 {
		g.lastReleased = input.Released
	}
	context.Clear()
	context.DrawText("P1.1 lifecycle/input parity", 12, 8)
	context.DrawText("Lifecycle: "+g.lifecycle, 12, 30)
	context.DrawText(g.lifecycleTraceLine(), 12, 52)
	context.DrawText(g.lifecycleCountLine(), 12, 74)
	context.DrawText(buttonLine(input), 12, 96)
	context.DrawText(g.edgeLine(), 12, 118)
	context.DrawText(crankLine(input), 12, 140)
	context.DrawText(dockLine(input), 12, 162)
	context.DrawText(deltaLine(input), 12, 184)
	context.DrawText(g.soakLine(), 12, 206)
	return true, nil
}

func (g *game) lifecycleTraceLine() string {
	line := "Trace:"
	for index := 0; index < g.traceLength; index++ {
		if index != 0 {
			line += ">"
		}
		line += lifecycleShortName(g.lifecycleTrace[index])
	}
	return line
}

func lifecycleShortName(event string) string {
	switch event {
	case "pause":
		return "P"
	case "resume":
		return "R"
	case "lock":
		return "L"
	case "unlock":
		return "U"
	case "terminate":
		return "T"
	case "low-power":
		return "LP"
	default:
		return "?"
	}
}

func (g *game) lifecycleCountLine() string {
	return "Count P/R:" + count(g.pauseCount) + "/" + count(g.resumeCount) +
		" L/U:" + count(g.lockCount) + "/" + count(g.unlockCount) +
		" LP:" + count(g.lowPowerCount)
}

func count(value uint32) string { return strconv.FormatUint(uint64(value), 10) }

func (g *game) edgeLine() string {
	return "Edges P*:" + buttonBits(g.lastPressed) + " R*:" + buttonBits(g.lastReleased)
}

func (g *game) soakLine() string {
	state := "RUN"
	if g.soakComplete {
		state = "DONE"
	}
	return "Soak:" + state + " " + decimal(g.elapsedSeconds) + "/60s"
}

func lifecycleName(event playdate.LifecycleEvent) string {
	switch event {
	case playdate.LifecyclePause:
		return "pause"
	case playdate.LifecycleResume:
		return "resume"
	case playdate.LifecycleLock:
		return "lock"
	case playdate.LifecycleUnlock:
		return "unlock"
	case playdate.LifecycleTerminate:
		return "terminate"
	case playdate.LifecycleLowPower:
		return "low-power"
	default:
		return "unknown"
	}
}

func buttonLine(input playdate.Input) string {
	return "Buttons C:" + buttonBits(input.Buttons) + " P:" + buttonBits(input.Pressed) +
		" R:" + buttonBits(input.Released) + " H:" + buttonBits(input.Held)
}

func buttonBits(buttons playdate.Buttons) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[buttons>>4], digits[buttons&0x0f]})
}

func crankLine(input playdate.Input) string {
	return "Crank A:" + decimal(input.CrankAngle) + " d:" + decimal(input.CrankDelta)
}

func dockLine(input playdate.Input) string {
	return "Docked:" + strconv.FormatBool(input.CrankDocked) +
		" +:" + strconv.FormatBool(input.CrankDockedThisFrame) +
		" -:" + strconv.FormatBool(input.CrankUndocked)
}

func deltaLine(input playdate.Input) string {
	return "Frame dt:" + decimal(input.DeltaSeconds*1000) + " ms"
}

func decimal(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', 2, 32)
}
