package systemcontrol

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type bitmap struct{ closed bool }

func (*bitmap) Width() (int, error)       { return 400, nil }
func (*bitmap) Height() (int, error)      { return 240, nil }
func (*bitmap) Clear() error              { return nil }
func (*bitmap) Fill(playdate.Color) error { return nil }
func (bitmap *bitmap) Close() error       { bitmap.closed = true; return nil }

type context struct {
	input            playdate.Input
	image            *bitmap
	callback         playdate.ButtonCallback
	lines            []string
	restarted        string
	autoLockDisabled bool
	crankMuted       bool
	menuOffset       int
}

func (*context) CurrentTimeMilliseconds() uint32 { return 0 }
func (*context) Clear()                          {}
func (context *context) DrawText(text string, _, _ int) {
	context.lines = append(context.lines, text)
}
func (*context) LoadBitmap(string) (playdate.Bitmap, error) { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error) {
	return nil, nil
}
func (context *context) NewBitmap(width, height int) (playdate.Bitmap, error) {
	context.image = &bitmap{}
	return context.image, nil
}
func (*context) DrawBitmap(playdate.Bitmap, int, int) error { return nil }
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error {
	return nil
}
func (*context) NewSprite() (playdate.Sprite, error) { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite {
	return nil
}
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (*context) UpdateAndDrawSprites()                                {}
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error) { return nil, nil }
func (*context) LoadFilePlayer(string) (playdate.FilePlayer, error)   { return nil, nil }
func (context *context) Input() playdate.Input                        { return context.input }
func (*context) DrawInto(_ playdate.Bitmap, callback func() error) error {
	return callback()
}
func (*context) LaunchArguments() (string, string) { return "cold", "/Games/p10.pdx" }
func (context *context) RestartGame(arguments string) error {
	context.restarted = arguments
	return nil
}
func (context *context) SetMenuImage(_ playdate.Bitmap, offset int) error {
	context.menuOffset = offset
	return nil
}
func (*context) ClearMenuImage() {}
func (context *context) SetAutoLockDisabled(disabled bool) {
	context.autoLockDisabled = disabled
}
func (context *context) SetCrankSoundsDisabled(disabled bool) bool {
	previous := context.crankMuted
	context.crankMuted = disabled
	return previous
}
func (context *context) SetButtonCallback(callback playdate.ButtonCallback, _ int) error {
	context.callback = callback
	return nil
}
func (*context) ButtonCallbackOverflow() uint32 { return 0 }

func TestAcceptanceFlowPreservesButtonEventsAndRestarts(t *testing.T) {
	probe := New().(*game)
	context := &context{}
	if err := probe.Init(context); err != nil {
		t.Fatal(err)
	}
	if context.menuOffset != 24 || !context.autoLockDisabled || !context.crankMuted || context.callback == nil {
		t.Fatalf("initial controls = offset %d, auto-lock %v, crank %v, callback %v", context.menuOffset, context.autoLockDisabled, context.crankMuted, context.callback != nil)
	}
	context.lines = nil
	context.callback(playdate.ButtonEvent{Button: playdate.ButtonA, Down: true, When: 100})
	context.callback(playdate.ButtonEvent{Button: playdate.ButtonA, Down: false, When: 101})
	if _, err := probe.Update(context); err != nil {
		t.Fatal(err)
	}
	if context.restarted != "p10-restarted" || probe.buttonTransitions != 2 {
		t.Fatalf("restart/transitions = %q/%d", context.restarted, probe.buttonTransitions)
	}
	if got := context.lines[4]; got != "BUTTON: 20 up @101 #2" {
		t.Fatalf("button line = %q", got)
	}
}

func TestMirrorLifecycleAndTerminateOwnership(t *testing.T) {
	probe := New().(*game)
	context := &context{}
	if err := probe.Init(context); err != nil {
		t.Fatal(err)
	}
	if err := probe.HandleLifecycle(context, playdate.LifecycleMirrorStarted); err != nil {
		t.Fatal(err)
	}
	if probe.mirrorState != "started" {
		t.Fatalf("mirror state = %q", probe.mirrorState)
	}
	if err := probe.HandleLifecycle(context, playdate.LifecycleMirrorEnded); err != nil {
		t.Fatal(err)
	}
	if probe.mirrorState != "ended" {
		t.Fatalf("mirror state = %q", probe.mirrorState)
	}
	if err := probe.HandleLifecycle(context, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if !context.image.closed {
		t.Fatal("menu image was not closed on termination")
	}
}
