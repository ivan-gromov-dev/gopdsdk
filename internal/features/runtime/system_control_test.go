package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestMenuImageStateValidatesAndRetainsOwnedBitmap(t *testing.T) {
	var calls []struct {
		handle uintptr
		offset int
	}
	state := NewMenuImageState(func(handle uintptr, offset int) {
		calls = append(calls, struct {
			handle uintptr
			offset int
		}{handle, offset})
	})
	freed := 0
	driver := BitmapDriver{
		Dimensions: func(uintptr) (int, int) { return 400, 240 },
		Free:       func(uintptr) { freed++ },
	}
	first := NewOwnedBitmap(11, driver)
	second := NewOwnedBitmap(12, driver)

	if err := state.SetMenuImage(first, -1); !errors.Is(err, playdate.ErrMenuImageOffset) {
		t.Fatalf("negative offset error = %v", err)
	}
	if err := state.SetMenuImage(first, 200); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); !errors.Is(err, playdate.ErrBitmapMenuImageInUse) {
		t.Fatalf("Close() retained image error = %v", err)
	}
	if err := state.SetMenuImage(second, 7); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("replaced image Close() error = %v", err)
	}
	state.ClearMenuImage()
	if err := second.Close(); err != nil {
		t.Fatalf("cleared image Close() error = %v", err)
	}
	if freed != 2 {
		t.Fatalf("freed = %d, want 2", freed)
	}
	want := []struct {
		handle uintptr
		offset int
	}{{11, 200}, {12, 7}, {0, 0}}
	if len(calls) != len(want) {
		t.Fatalf("native calls = %v, want %v", calls, want)
	}
	for index := range want {
		if calls[index] != want[index] {
			t.Fatalf("native call %d = %v, want %v", index, calls[index], want[index])
		}
	}
}

func TestMenuImageStateRejectsWrongSizeAndBorrowedBitmap(t *testing.T) {
	state := NewMenuImageState(func(uintptr, int) {})
	wrongSize := NewOwnedBitmap(1, BitmapDriver{Dimensions: func(uintptr) (int, int) { return 200, 240 }})
	if err := state.SetMenuImage(wrongSize, 0); !errors.Is(err, playdate.ErrMenuImageSize) {
		t.Fatalf("wrong size error = %v", err)
	}
	borrowed := NewBorrowedBitmap(2, BitmapDriver{Dimensions: func(uintptr) (int, int) { return 400, 240 }})
	if err := state.SetMenuImage(borrowed, 0); !errors.Is(err, playdate.ErrBitmapBorrowed) {
		t.Fatalf("borrowed image error = %v", err)
	}
}

func TestSystemControlValidation(t *testing.T) {
	if err := ValidateLaunchArguments("restart\x00truncated"); !errors.Is(err, playdate.ErrLaunchArguments) {
		t.Fatalf("launch arguments error = %v", err)
	}
	if err := ValidateLaunchArguments("restart-mode"); err != nil {
		t.Fatal(err)
	}
	callback := func(playdate.ButtonEvent) {}
	for _, test := range []struct {
		callback playdate.ButtonCallback
		queue    int
		valid    bool
	}{{nil, 0, true}, {callback, 1, true}, {callback, playdate.ButtonCallbackQueueLimit, true}, {nil, 1, false}, {callback, 0, false}, {callback, playdate.ButtonCallbackQueueLimit + 1, false}} {
		err := ValidateButtonCallbackConfig(test.callback, test.queue)
		if (err == nil) != test.valid {
			t.Errorf("ValidateButtonCallbackConfig(callback=%v, queue=%d) error = %v, valid=%v", test.callback != nil, test.queue, err, test.valid)
		}
	}
}

type systemControlContext struct {
	testContext
	calls          []string
	callback       playdate.ButtonCallback
	crankDisabled  bool
	menuCleared    bool
	buttonOverflow uint32
}

func (context *systemControlContext) LaunchArguments() (string, string) {
	return "cold-start", "/Games/control.pdx"
}
func (context *systemControlContext) RestartGame(arguments string) error {
	context.calls = append(context.calls, "restart:"+arguments)
	return nil
}
func (context *systemControlContext) SetMenuImage(playdate.Bitmap, int) error {
	context.menuCleared = false
	context.calls = append(context.calls, "menu:set")
	return nil
}
func (context *systemControlContext) ClearMenuImage() {
	context.menuCleared = true
	context.calls = append(context.calls, "menu:clear")
}
func (context *systemControlContext) SetAutoLockDisabled(disabled bool) {
	context.calls = append(context.calls, "autolock:"+boolText(disabled))
}
func (context *systemControlContext) SetCrankSoundsDisabled(disabled bool) bool {
	previous := context.crankDisabled
	context.crankDisabled = disabled
	context.calls = append(context.calls, "crank:"+boolText(disabled))
	return previous
}
func (context *systemControlContext) SetButtonCallback(callback playdate.ButtonCallback, queueSize int) error {
	context.callback = callback
	context.calls = append(context.calls, "buttons:"+boolText(callback != nil))
	return nil
}
func (context *systemControlContext) ButtonCallbackOverflow() uint32 {
	return context.buttonOverflow
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

type systemControlGame struct {
	context          *systemControlContext
	lifecycle        []playdate.LifecycleEvent
	terminateCleaned bool
}

type systemControlBitmap struct{}

func (systemControlBitmap) Width() (int, error)  { return 400, nil }
func (systemControlBitmap) Height() (int, error) { return 240, nil }
func (systemControlBitmap) Clear() error         { return nil }
func (systemControlBitmap) Fill(playdate.Color) error {
	return nil
}
func (systemControlBitmap) Close() error { return nil }

func (game *systemControlGame) Init(context playdate.Context) error {
	controls := context.(playdate.SystemControls)
	arguments, path := controls.LaunchArguments()
	game.context.calls = append(game.context.calls, arguments+":"+path)
	if err := controls.RestartGame("warm-start"); err != nil {
		return err
	}
	if err := controls.SetMenuImage(systemControlBitmap{}, 20); err != nil {
		return err
	}
	controls.SetAutoLockDisabled(true)
	controls.SetCrankSoundsDisabled(false)
	if err := controls.SetButtonCallback(func(playdate.ButtonEvent) {}, 8); err != nil {
		return err
	}
	game.context.buttonOverflow = 3
	if got := controls.ButtonCallbackOverflow(); got != 3 {
		return errors.New("button callback overflow was not forwarded")
	}
	return nil
}
func (*systemControlGame) Update(playdate.Context) (bool, error) { return false, nil }
func (game *systemControlGame) HandleLifecycle(context playdate.Context, event playdate.LifecycleEvent) error {
	game.lifecycle = append(game.lifecycle, event)
	if event == playdate.LifecycleTerminate {
		game.terminateCleaned = game.context.menuCleared && game.context.callback == nil
		if err := context.(playdate.SystemControls).RestartGame("late"); !errors.Is(err, playdate.ErrSystemControlsUnavailable) {
			return errors.New("system controls remained available during termination")
		}
	}
	return nil
}

func TestApplicationForwardsSystemControlsAndMirrorLifecycle(t *testing.T) {
	context := &systemControlContext{crankDisabled: true}
	game := &systemControlGame{context: context}
	application, err := NewApplication(game, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{EventInit, EventMirrorStarted, EventMirrorEnded, EventTerminate} {
		if err := application.Handle(event, 0); err != nil {
			t.Fatal(err)
		}
	}
	wantLifecycle := []playdate.LifecycleEvent{playdate.LifecycleMirrorStarted, playdate.LifecycleMirrorEnded, playdate.LifecycleTerminate}
	if len(game.lifecycle) != len(wantLifecycle) {
		t.Fatalf("lifecycle = %v, want %v", game.lifecycle, wantLifecycle)
	}
	for index := range wantLifecycle {
		if game.lifecycle[index] != wantLifecycle[index] {
			t.Fatalf("lifecycle = %v, want %v", game.lifecycle, wantLifecycle)
		}
	}
	if !game.terminateCleaned {
		t.Fatal("menu image and button callback were not cleared before LifecycleTerminate")
	}
	if !context.crankDisabled {
		t.Fatal("initial crank-sound state was not restored")
	}
	wantCalls := []string{
		"cold-start:/Games/control.pdx", "restart:warm-start", "menu:set",
		"autolock:true", "crank:false", "buttons:true", "buttons:false",
		"menu:clear", "autolock:false", "crank:true",
	}
	if len(context.calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", context.calls, wantCalls)
	}
	for index := range wantCalls {
		if context.calls[index] != wantCalls[index] {
			t.Fatalf("calls = %v, want %v", context.calls, wantCalls)
		}
	}
}
