package runtime

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestOwnedFontMeasurementAndClose(t *testing.T) {
	freed := uintptr(0)
	font := NewOwnedFont(17, FontDriver{
		TextWidth: func(handle uintptr, text string) int { return int(handle) + len(text) },
		Height:    func(handle uintptr) int { return int(handle) },
		Free:      func(handle uintptr) { freed = handle },
	})
	if width, err := font.TextWidth("HUD"); err != nil || width != 20 {
		t.Fatalf("TextWidth = %d, %v", width, err)
	}
	if height, err := font.Height(); err != nil || height != 17 {
		t.Fatalf("Height = %d, %v", height, err)
	}
	if err := font.Close(); err != nil || freed != 17 {
		t.Fatalf("Close = %v, freed %d", err, freed)
	}
	if _, err := font.TextWidth("HUD"); !errors.Is(err, playdate.ErrFontClosed) {
		t.Fatalf("closed TextWidth = %v", err)
	}
	if err := font.Close(); !errors.Is(err, playdate.ErrFontClosed) {
		t.Fatalf("second Close = %v", err)
	}
}

func TestOnePoleAcceptsLowAndHighPassRange(t *testing.T) {
	var got float32
	effect := NewOnePoleFilter(7, OnePoleFilterDriver{Effect: EffectDriver{SetMix: func(uintptr, float32) {}, SetMixModulator: func(uintptr, uintptr) {}, Free: func(uintptr) {}}, SetParameter: func(_ uintptr, value float32) { got = value }, SetParameterModulator: func(uintptr, uintptr) {}})
	for _, value := range []float32{-1, 1} {
		if err := effect.SetParameter(value); err != nil || got != value {
			t.Fatalf("SetParameter(%v) = %v, got %v", value, err, got)
		}
	}
	if err := effect.SetParameter(-1.01); err != playdate.ErrAudioParameter {
		t.Fatalf("SetParameter(-1.01) = %v", err)
	}
}

func TestFontHandleRejectsForeignFont(t *testing.T) {
	if _, err := FontHandle(foreignFont{}); !errors.Is(err, playdate.ErrFontInvalid) {
		t.Fatalf("FontHandle = %v", err)
	}
}

func TestBorrowedGlyphBitmapExpiresWithFont(t *testing.T) {
	owner := NewOwnedFont(17, FontDriver{Free: func(uintptr) {}})
	bitmap, err := NewBorrowedFontBitmap(owner, 23, BitmapDriver{Dimensions: func(uintptr) (int, int) { return 5, 7 }})
	if err != nil {
		t.Fatal(err)
	}
	if width, widthErr := bitmap.Width(); widthErr != nil || width != 5 {
		t.Fatalf("Width = %d, %v", width, widthErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := bitmap.Width(); !errors.Is(err, playdate.ErrBitmapClosed) {
		t.Fatalf("expired Width error = %v", err)
	}
}

type foreignFont struct{}

func (foreignFont) TextWidth(string) (int, error) { return 0, nil }
func (foreignFont) Height() (int, error)          { return 0, nil }
func (foreignFont) Close() error                  { return nil }

type testContext struct{}

func (testContext) Clear()                                                             {}
func (testContext) DrawText(string, int, int)                                          {}
func (testContext) LoadFont(string) (playdate.Font, error)                             { return foreignFont{}, nil }
func (testContext) DrawTextFont(playdate.Font, string, int, int) error                 { return nil }
func (testContext) CurrentTimeMilliseconds() uint32                                    { return 0 }
func (testContext) Input() playdate.Input                                              { return playdate.Input{} }
func (testContext) LoadBitmap(string) (playdate.Bitmap, error)                         { return nil, nil }
func (testContext) LoadBitmapTable(string) (playdate.BitmapTable, error)               { return nil, nil }
func (testContext) NewBitmap(int, int) (playdate.Bitmap, error)                        { return nil, nil }
func (testContext) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (testContext) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (testContext) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (testContext) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (testContext) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (testContext) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) {
	return nil, nil
}
func (testContext) UpdateAndDrawSprites()                                {}
func (testContext) LoadSoundEffect(string) (playdate.SoundEffect, error) { return nil, nil }
func (testContext) LoadFilePlayer(string) (playdate.FilePlayer, error)   { return nil, nil }

type launcherContext struct {
	testContext
	exited bool
}

func (context *launcherContext) ExitToLauncher() { context.exited = true }

type graphicsCapabilityContext struct {
	testContext
	draws, framebuffers, offscreen, transformed, stencils int
}

func (c *graphicsCapabilityContext) DrawLine(int, int, int, int, int, playdate.Paint) error {
	c.draws++
	return nil
}
func (*graphicsCapabilityContext) DrawRect(int, int, int, int, playdate.Paint) error { return nil }
func (*graphicsCapabilityContext) FillRect(int, int, int, int, playdate.Paint) error { return nil }
func (*graphicsCapabilityContext) DrawEllipse(int, int, int, int, int, float32, float32, playdate.Paint) error {
	return nil
}
func (*graphicsCapabilityContext) FillEllipse(int, int, int, int, float32, float32, playdate.Paint) error {
	return nil
}
func (*graphicsCapabilityContext) DrawTriangle(int, int, int, int, int, int, int, playdate.Paint) error {
	return nil
}
func (*graphicsCapabilityContext) FillTriangle(int, int, int, int, int, int, playdate.Paint) error {
	return nil
}
func (*graphicsCapabilityContext) FillPolygon([]playdate.GraphicsPoint, playdate.PolygonFillRule, playdate.Paint) error {
	return nil
}
func (*graphicsCapabilityContext) DrawRoundedRect(int, int, int, int, int, int, playdate.Paint) error {
	return nil
}
func (*graphicsCapabilityContext) FillRoundedRect(int, int, int, int, int, playdate.Paint) error {
	return nil
}
func (*graphicsCapabilityContext) SetClipRect(int, int, int, int) error        { return nil }
func (*graphicsCapabilityContext) ClearClipRect()                              {}
func (*graphicsCapabilityContext) SetDrawOffset(int, int)                      {}
func (*graphicsCapabilityContext) SetDrawMode(playdate.DrawMode) error         { return nil }
func (*graphicsCapabilityContext) SetLineCapStyle(playdate.LineCapStyle) error { return nil }
func (*graphicsCapabilityContext) SetBackgroundColor(playdate.Color) error     { return nil }
func (*graphicsCapabilityContext) SetScreenClipRect(int, int, int, int) error  { return nil }
func (c *graphicsCapabilityContext) WithFramebuffer(callback func(playdate.Framebuffer) error) error {
	c.framebuffers++
	return callback(nil)
}
func (c *graphicsCapabilityContext) DrawInto(_ playdate.Bitmap, callback func() error) error {
	c.offscreen++
	return callback()
}
func (c *graphicsCapabilityContext) DrawRotatedBitmap(playdate.Bitmap, int, int, float32, float32, float32, float32, float32) error {
	c.transformed++
	return nil
}
func (c *graphicsCapabilityContext) WithStencil(_ playdate.Bitmap, _ bool, callback func() error) error {
	c.stencils++
	return callback()
}

type graphicsCapabilityGame struct{}

func (graphicsCapabilityGame) Init(context playdate.Context) error {
	if _, ok := context.(playdate.PrimitiveGraphics); !ok {
		return playdate.ErrGraphicsUnavailable
	}
	if _, ok := context.(playdate.GraphicsState); !ok {
		return playdate.ErrGraphicsUnavailable
	}
	if _, ok := context.(playdate.FramebufferGraphics); !ok {
		return playdate.ErrGraphicsUnavailable
	}
	if _, ok := context.(playdate.OffscreenGraphics); !ok {
		return playdate.ErrGraphicsUnavailable
	}
	if _, ok := context.(playdate.BitmapCompositor); !ok {
		return playdate.ErrGraphicsUnavailable
	}
	return nil
}
func (graphicsCapabilityGame) Update(context playdate.Context) (bool, error) {
	paint, _ := playdate.SolidPaint(playdate.ColorBlack)
	if err := context.(playdate.PrimitiveGraphics).DrawLine(0, 0, 1, 1, 1, paint); err != nil {
		return false, err
	}
	if err := context.(playdate.FramebufferGraphics).WithFramebuffer(func(playdate.Framebuffer) error { return nil }); err != nil {
		return false, err
	}
	if err := context.(playdate.BitmapCompositor).DrawRotatedBitmap(nil, 10, 20, 45, .5, .5, 1, 1); err != nil {
		return false, err
	}
	if err := context.(playdate.BitmapCompositor).WithStencil(nil, false, func() error {
		return context.(playdate.BitmapCompositor).WithStencil(nil, false, func() error { return nil })
	}); !errors.Is(err, playdate.ErrGraphicsStencilActive) {
		return false, err
	}
	if err := context.(playdate.BitmapCompositor).WithStencil(nil, false, func() error { return nil }); err != nil {
		return false, err
	}
	return true, context.(playdate.OffscreenGraphics).DrawInto(nil, func() error { return nil })
}

func TestApplicationForwardsOptionalGraphicsCapabilities(t *testing.T) {
	context := &graphicsCapabilityContext{}
	application, err := NewApplication(graphicsCapabilityGame{}, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if refresh, err := application.Update(RawInput{}); err != nil || refresh != 1 {
		t.Fatalf("Update() = %v, %v", refresh, err)
	}
	if context.draws != 1 {
		t.Fatalf("draws = %d", context.draws)
	}
	if context.framebuffers != 1 || context.offscreen != 1 {
		t.Fatalf("framebuffers/offscreen = %d/%d", context.framebuffers, context.offscreen)
	}
	if context.transformed != 1 || context.stencils != 2 {
		t.Fatalf("transformed/stencils = %d/%d", context.transformed, context.stencils)
	}
}

func TestAudioOwnershipStateAndValidation(t *testing.T) {
	playing := false
	paused := false
	freed := 0
	left, right := float32(1), float32(1)
	driver := AudioDriver{
		Play:      func(uintptr) bool { playing = true; return true },
		Stop:      func(uintptr) { playing = false },
		SetVolume: func(_ uintptr, l, r float32) { left, right = l, r },
		Volume:    func(uintptr) (float32, float32) { return left, right },
		IsPlaying: func(uintptr) bool { return playing },
		Pause:     func(_ uintptr, value bool) { paused = value },
		Free:      func(uintptr) { freed++ },
	}
	player := NewSoundEffect(17, driver)
	if err := player.SetVolume(.25, .75); err != nil {
		t.Fatal(err)
	}
	if l, r, err := player.Volume(); err != nil || l != .25 || r != .75 {
		t.Fatalf("Volume() = %v, %v, %v", l, r, err)
	}
	if err := player.Play(); err != nil {
		t.Fatal(err)
	}
	if state, _ := player.State(); state != playdate.PlaybackPlaying {
		t.Fatalf("state = %v", state)
	}
	if err := player.Pause(); err != nil || !paused {
		t.Fatalf("Pause() = %v, paused %v", err, paused)
	}
	if state, _ := player.State(); state != playdate.PlaybackPaused {
		t.Fatalf("state = %v", state)
	}
	if err := player.Resume(); err != nil || paused {
		t.Fatalf("Resume() = %v, paused %v", err, paused)
	}
	if err := player.Stop(); err != nil {
		t.Fatal(err)
	}
	if state, _ := player.State(); state != playdate.PlaybackStopped {
		t.Fatalf("state = %v", state)
	}
	if err := player.Play(); err != nil {
		t.Fatal(err)
	}
	if err := player.Close(); err != nil || freed != 1 || playing {
		t.Fatalf("Close() = %v, freed %d, playing %v", err, freed, playing)
	}
	if err := player.Play(); !errors.Is(err, playdate.ErrAudioClosed) {
		t.Fatalf("closed Play() = %v", err)
	}
	if err := ValidateAudioVolume(-1, 1); !errors.Is(err, playdate.ErrAudioVolume) {
		t.Fatalf("volume error = %v", err)
	}
}

func TestAudioPlayFailure(t *testing.T) {
	player := NewFilePlayer(1, AudioDriver{Play: func(uintptr) bool { return false }})
	if err := player.Play(); !errors.Is(err, playdate.ErrAudioPlay) {
		t.Fatalf("Play() = %v", err)
	}
}

func TestFilePlayerVariableRate(t *testing.T) {
	rate := float32(1)
	player := NewFilePlayer(1, AudioDriver{
		SetRate: func(_ uintptr, value float32) { rate = value },
		Rate:    func(uintptr) float32 { return rate },
	})
	variable, ok := player.(playdate.VariableRatePlayer)
	if !ok {
		t.Fatal("FilePlayer does not expose VariableRatePlayer")
	}
	if err := variable.SetRate(.75); err != nil {
		t.Fatal(err)
	}
	if value, err := variable.Rate(); err != nil || value != .75 {
		t.Fatalf("Rate() = %v, %v", value, err)
	}
	if err := variable.SetRate(-1); !errors.Is(err, playdate.ErrAudioReverseUnsupported) {
		t.Fatalf("reverse file rate = %v", err)
	}
	if rate != .75 {
		t.Fatalf("native rate changed after rejected reverse = %v", rate)
	}
}

func TestAudioCompletionAndFadeCallbacks(t *testing.T) {
	var finishID, fadeID uint32
	player := NewFilePlayer(5, AudioDriver{
		SetFinishCallback: func(_ uintptr, callback uint32) { finishID = callback },
		FadeVolume: func(_ uintptr, left, right float32, frames, callback uint32) {
			if left != .25 || right != .75 || frames != 4410 {
				t.Fatalf("fade = %v, %v, %d", left, right, frames)
			}
			fadeID = callback
		},
		Stop: func(uintptr) {}, Free: func(uintptr) {},
	})
	completed, faded := 0, 0
	if err := player.(playdate.CompletionPlayer).SetFinishCallback(func() { completed++ }); err != nil {
		t.Fatal(err)
	}
	if finishID == 0 {
		t.Fatal("finish callback was not registered")
	}
	InvokeAudioCallback(finishID, false)
	InvokeAudioCallback(finishID, false)
	DrainAudioCallbacks()
	if completed != 2 {
		t.Fatalf("completed = %d", completed)
	}
	oldFinishID := finishID
	if err := player.(playdate.CompletionPlayer).SetFinishCallback(func() { completed += 10 }); err != nil {
		t.Fatal(err)
	}
	InvokeAudioCallback(oldFinishID, false)
	InvokeAudioCallback(finishID, false)
	DrainAudioCallbacks()
	if completed != 12 {
		t.Fatalf("completed after replacement = %d", completed)
	}
	if err := player.(playdate.FadingPlayer).FadeVolume(.25, .75, 4410, func() { faded++ }); err != nil {
		t.Fatal(err)
	}
	InvokeAudioCallback(fadeID, true)
	InvokeAudioCallback(fadeID, true)
	DrainAudioCallbacks()
	if faded != 1 {
		t.Fatalf("faded = %d", faded)
	}
	if err := player.(playdate.FadingPlayer).FadeVolume(1, 1, 2147483648, nil); !errors.Is(err, playdate.ErrAudioFade) {
		t.Fatalf("large fade = %v", err)
	}
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
	if finishID != 0 {
		t.Fatalf("native finish callback after Close = %d", finishID)
	}
}

func TestSamplePlayerDoesNotExposeFadingPlayer(t *testing.T) {
	player := NewSamplePlayer(1, AudioDriver{})
	if _, ok := player.(playdate.FadingPlayer); ok {
		t.Fatal("SamplePlayer exposes streaming-only fades")
	}
	if _, ok := player.(playdate.CompletionPlayer); !ok {
		t.Fatal("SamplePlayer does not expose completion callbacks")
	}
}

type audioClockContext struct{ testContext }

func (audioClockContext) CurrentAudioTime() (uint32, error) { return 12345, nil }

func TestApplicationForwardsAudioClock(t *testing.T) {
	application, err := NewApplication(testGame{init: func(context playdate.Context) error {
		value, clockErr := context.(playdate.AudioClock).CurrentAudioTime()
		if value != 12345 || clockErr != nil {
			t.Fatalf("CurrentAudioTime = %d, %v", value, clockErr)
		}
		return nil
	}}, audioClockContext{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSamplePlayerControls(t *testing.T) {
	playingRepeat := 0
	playingRate := float32(0)
	offset := float32(0)
	rate := float32(1)
	player := NewSamplePlayer(9, AudioDriver{
		PlayRepeated: func(_ uintptr, repeat int, value float32) bool {
			playingRepeat, playingRate = repeat, value
			return true
		},
		Length:    func(uintptr) float32 { return 2.5 },
		SetOffset: func(_ uintptr, value float32) { offset = value }, Offset: func(uintptr) float32 { return offset },
		SetRate: func(_ uintptr, value float32) { rate = value }, Rate: func(uintptr) float32 { return rate },
		Stop: func(uintptr) {}, Free: func(uintptr) {},
	})
	if err := player.PlayRepeated(3, .5); err != nil || playingRepeat != 3 || playingRate != .5 {
		t.Fatalf("PlayRepeated() = %v, %d, %v", err, playingRepeat, playingRate)
	}
	if length, err := player.Length(); err != nil || length != 2.5 {
		t.Fatalf("Length() = %v, %v", length, err)
	}
	if err := player.SetOffset(1.25); err != nil {
		t.Fatal(err)
	}
	if value, err := player.Offset(); err != nil || value != 1.25 {
		t.Fatalf("Offset() = %v, %v", value, err)
	}
	if err := player.SetRate(-1); err != nil {
		t.Fatal(err)
	}
	if value, err := player.Rate(); err != nil || value != -1 {
		t.Fatalf("Rate() = %v, %v", value, err)
	}
	if err := player.PlayRepeated(-1, 1); !errors.Is(err, playdate.ErrAudioRepeat) {
		t.Fatalf("negative repeat = %v", err)
	}
	if int(^uint(0)>>63) == 1 {
		if err := player.PlayRepeated(int(int64(2147483648)), 1); !errors.Is(err, playdate.ErrAudioRepeat) {
			t.Fatalf("large repeat = %v", err)
		}
	}
	if err := player.SetRate(0); !errors.Is(err, playdate.ErrAudioRate) {
		t.Fatalf("zero rate = %v", err)
	}
	if err := player.SetOffset(-1); !errors.Is(err, playdate.ErrAudioOffset) {
		t.Fatalf("negative offset = %v", err)
	}
}

type samplePlayerContext struct {
	testContext
	loaded string
}

func (context *samplePlayerContext) LoadSamplePlayer(path string) (playdate.SamplePlayer, error) {
	context.loaded = path
	return NewSamplePlayer(1, AudioDriver{Stop: func(uintptr) {}, Free: func(uintptr) {}}), nil
}

func TestApplicationForwardsSamplePlayers(t *testing.T) {
	context := &samplePlayerContext{}
	application, err := NewApplication(testGame{init: func(got playdate.Context) error {
		samples, ok := got.(playdate.SamplePlayers)
		if !ok {
			return errors.New("sample players not forwarded")
		}
		_, loadErr := samples.LoadSamplePlayer("audio/hit")
		return loadErr
	}}, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if context.loaded != "audio/hit" {
		t.Fatalf("loaded = %q", context.loaded)
	}
}

func TestAudioChannelOwnership(t *testing.T) {
	var added, removed, freed int
	source := NewSamplePlayer(7, AudioDriver{Source: func(uintptr) uintptr { return 70 }, Stop: func(uintptr) {}, Free: func(uintptr) {}})
	channel := NewAudioChannel(9, AudioChannelDriver{
		AddSource:    func(channel, source uintptr) bool { added++; return channel == 9 && source == 70 },
		RemoveSource: func(channel, source uintptr) bool { removed++; return channel == 9 && source == 70 },
		SetVolume:    func(uintptr, float32) {}, Volume: func(uintptr) float32 { return .5 }, SetPan: func(uintptr, float32) {},
		Remove: func(uintptr) bool { return true }, Free: func(uintptr) { freed++ },
	})
	if err := channel.AddSource(source); err != nil {
		t.Fatal(err)
	}
	if err := channel.AddSource(source); err != nil || added != 2 {
		t.Fatalf("refresh AddSource = %v, %d", err, added)
	}
	if err := source.Close(); err != nil || removed != 1 {
		t.Fatalf("source Close = %v, removed %d", err, removed)
	}
	if err := channel.Close(); err != nil || freed != 1 {
		t.Fatalf("channel Close = %v, freed %d", err, freed)
	}
	if err := channel.SetPan(0); !errors.Is(err, playdate.ErrAudioChannelClosed) {
		t.Fatalf("closed SetPan = %v", err)
	}
}

func TestDefaultAudioChannelDoesNotRemoveOrFreeNativeChannel(t *testing.T) {
	removed, freed := 0, 0
	channel := DefaultAudioChannel(9, AudioChannelDriver{
		Remove: func(uintptr) bool { removed++; return true },
		Free:   func(uintptr) { freed++ },
	})
	if err := channel.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if removed != 0 || freed != 0 {
		t.Fatalf("borrowed default channel removed %d times, freed %d times", removed, freed)
	}
}

func TestAudioChannelOutputRoutingRejectsCyclesAndExpiresOnClose(t *testing.T) {
	var added, removed [][2]uintptr
	driver := AudioChannelDriver{
		Output:      func(channel uintptr) uintptr { return channel + 100 },
		OutputAudio: AudioDriver{Source: func(handle uintptr) uintptr { return handle }, Stop: func(uintptr) {}, SetVolume: func(uintptr, float32, float32) {}, Volume: func(uintptr) (float32, float32) { return 1, 1 }, IsPlaying: func(uintptr) bool { return true }, Pause: func(uintptr, bool) {}},
		AddSource: func(channel, source uintptr) bool {
			added = append(added, [2]uintptr{channel, source})
			return true
		},
		RemoveSource: func(channel, source uintptr) bool {
			removed = append(removed, [2]uintptr{channel, source})
			return true
		},
		Remove: func(uintptr) bool { return true }, Free: func(uintptr) {},
	}
	first := NewAudioChannel(1, driver)
	second := NewAudioChannel(2, driver)
	third := NewAudioChannel(3, driver)
	firstOutput, err := first.Output()
	if err != nil {
		t.Fatal(err)
	}
	if again, err := first.Output(); err != nil || again != firstOutput {
		t.Fatalf("cached Output() = %v, %v", again, err)
	}
	if err := second.AddSource(firstOutput); err != nil {
		t.Fatal(err)
	}
	secondOutput, err := second.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := third.AddSource(secondOutput); err != nil {
		t.Fatal(err)
	}
	thirdOutput, err := third.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.AddSource(thirdOutput); !errors.Is(err, playdate.ErrAudioRoute) {
		t.Fatalf("cycle AddSource = %v", err)
	}
	if err := first.AddSource(firstOutput); !errors.Is(err, playdate.ErrAudioRoute) {
		t.Fatalf("self AddSource = %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("native adds = %v", added)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != [2]uintptr{2, 101} {
		t.Fatalf("owner Close removals = %v", removed)
	}
	if _, _, err := firstOutput.Volume(); !errors.Is(err, playdate.ErrAudioClosed) {
		t.Fatalf("expired output Volume = %v", err)
	}
}

func TestAudioChannelLevelSignalsAreBorrowedAndExpireWithOwner(t *testing.T) {
	var volumeModulator, panModulator uintptr
	freed := 0
	driver := AudioChannelDriver{
		DryLevelSignal: func(channel uintptr) uintptr { return channel + 200 },
		WetLevelSignal: func(channel uintptr) uintptr { return channel + 300 },
		LevelSignal: SignalDriver{
			Value:    func(handle uintptr) float32 { return float32(handle) / 1000 },
			SetScale: func(uintptr, float32) {}, SetOffset: func(uintptr, float32) {},
			Free: func(uintptr) { freed++ },
		},
		SetVolumeModulator: func(_ uintptr, signal uintptr) { volumeModulator = signal },
		SetPanModulator:    func(_ uintptr, signal uintptr) { panModulator = signal },
		Remove:             func(uintptr) bool { return true }, Free: func(uintptr) {},
	}
	owner := NewAudioChannel(1, driver)
	consumer := NewAudioChannel(2, driver)
	dry, err := owner.DryLevelSignal()
	if err != nil {
		t.Fatal(err)
	}
	if value, err := dry.Value(); err != nil || value != .201 {
		t.Fatalf("dry Value = %v, %v", value, err)
	}
	if err := dry.Close(); err != nil || freed != 0 {
		t.Fatalf("borrowed dry Close = %v, frees %d", err, freed)
	}
	reopenedDry, err := owner.DryLevelSignal()
	if err != nil || reopenedDry == dry {
		t.Fatalf("reopened dry = %v, %v", reopenedDry, err)
	}
	wet, err := owner.WetLevelSignal()
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.SetVolumeModulator(reopenedDry); err != nil || volumeModulator != 201 {
		t.Fatalf("dry volume modulator = %v, %d", err, volumeModulator)
	}
	if err := consumer.SetPanModulator(wet); err != nil || panModulator != 301 {
		t.Fatalf("wet pan modulator = %v, %d", err, panModulator)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if volumeModulator != 0 || panModulator != 0 || freed != 0 {
		t.Fatalf("owner Close modulators/frees = %d, %d, %d", volumeModulator, panModulator, freed)
	}
	if _, err := reopenedDry.Value(); !errors.Is(err, playdate.ErrAudioGraphClosed) {
		t.Fatalf("expired dry Value = %v", err)
	}
}

type audioOutputContext struct {
	testContext
	channel playdate.AudioChannel
	set     [2]bool
}

func (c *audioOutputContext) DefaultAudioChannel() (playdate.AudioChannel, error) {
	return c.channel, nil
}
func (*audioOutputContext) AudioOutputState() (playdate.AudioOutputState, error) {
	return playdate.AudioOutputState{Headphones: true, HeadsetMicrophone: true}, nil
}
func (c *audioOutputContext) SetAudioOutputsActive(headphones, speaker bool) error {
	c.set = [2]bool{headphones, speaker}
	return nil
}

func TestApplicationForwardsAudioOutputs(t *testing.T) {
	native := &audioOutputContext{channel: DefaultAudioChannel(7, AudioChannelDriver{
		Output:      func(uintptr) uintptr { return 17 },
		OutputAudio: AudioDriver{Source: func(h uintptr) uintptr { return h }, Stop: func(uintptr) {}, SetVolume: func(uintptr, float32, float32) {}, Volume: func(uintptr) (float32, float32) { return 1, 1 }, IsPlaying: func(uintptr) bool { return true }, Pause: func(uintptr, bool) {}},
	})}
	application, err := NewApplication(testGame{init: func(context playdate.Context) error {
		outputs, ok := context.(playdate.AudioOutputs)
		if !ok {
			return errors.New("audio outputs not forwarded")
		}
		channel, err := outputs.DefaultAudioChannel()
		if err != nil || channel == nil {
			t.Fatalf("DefaultAudioChannel() = %v, %v", channel, err)
		}
		output, err := channel.Output()
		if err != nil || output == nil {
			t.Fatalf("Output() = %v, %v", output, err)
		}
		state, err := outputs.AudioOutputState()
		if err != nil || !state.Headphones || !state.HeadsetMicrophone {
			t.Fatalf("AudioOutputState() = %#v, %v", state, err)
		}
		return outputs.SetAudioOutputsActive(true, false)
	}}, native, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if native.set != [2]bool{true, false} {
		t.Fatalf("SetAudioOutputsActive forwarded %v", native.set)
	}
}

func TestSynthSignalOwnership(t *testing.T) {
	var frequency, amplitude uintptr
	var signalFreed, synthFreed int
	signalDriver := SignalDriver{Value: func(uintptr) float32 { return .25 }, SetScale: func(uintptr, float32) {}, SetOffset: func(uintptr, float32) {}, Free: func(uintptr) { signalFreed++ }}
	lfo := NewLFO(3, LFODriver{Signal: signalDriver, SetRate: func(uintptr, float32) {}, SetPhase: func(uintptr, float32) {}, SetCenter: func(uintptr, float32) {}, SetDepth: func(uintptr, float32) {}, SetRetrigger: func(uintptr, bool) {}})
	synth := NewSynth(4, SynthDriver{
		Audio:       AudioDriver{Source: func(handle uintptr) uintptr { return handle }, Stop: func(uintptr) {}, Free: func(uintptr) { synthFreed++ }},
		SetWaveform: func(uintptr, playdate.Waveform) {}, SetEnvelope: func(uintptr, float32, float32, float32, float32) {}, SetTranspose: func(uintptr, float32) {},
		SetFrequencyModulator: func(_ uintptr, signal uintptr) { frequency = signal }, SetAmplitudeModulator: func(_ uintptr, signal uintptr) { amplitude = signal },
		PlayMIDINote: func(uintptr, float32, float32, float32, uint32) {}, NoteOff: func(uintptr, uint32) {},
	})
	if err := synth.SetFrequencyModulator(lfo); err != nil || frequency != 3 {
		t.Fatalf("SetFrequencyModulator = %v, %d", err, frequency)
	}
	if err := synth.SetAmplitudeModulator(lfo); err != nil || amplitude != 3 {
		t.Fatalf("SetAmplitudeModulator = %v, %d", err, amplitude)
	}
	if err := lfo.Close(); err != nil || frequency != 0 || amplitude != 0 || signalFreed != 1 {
		t.Fatalf("LFO Close = %v, %d, %d, %d", err, frequency, amplitude, signalFreed)
	}
	if err := synth.PlayMIDINote(60, 1, .5, 100); err != nil {
		t.Fatal(err)
	}
	if err := synth.PlayMIDINote(60, 2, .5, 100); !errors.Is(err, playdate.ErrAudioParameter) {
		t.Fatalf("velocity = %v", err)
	}
	if err := synth.Close(); err != nil || synthFreed != 1 {
		t.Fatalf("Synth Close = %v, %d", err, synthFreed)
	}
}

func TestLFOAndEnvelopeCompletionControls(t *testing.T) {
	var start float32
	var seed uint16
	var lfoGlobal bool
	lfo := NewLFO(3, LFODriver{
		Signal:        SignalDriver{Free: func(uintptr) {}},
		SetStartPhase: func(_ uintptr, value float32) { start = value },
		SetRandomSeed: func(_ uintptr, value uint16) { seed = value },
		SetGlobal:     func(_ uintptr, value bool) { lfoGlobal = value },
	})
	if err := lfo.SetStartPhase(.25); err != nil || start != .25 {
		t.Fatalf("SetStartPhase = %v, %v", err, start)
	}
	if err := lfo.SetStartPhase(-.1); !errors.Is(err, playdate.ErrAudioParameter) {
		t.Fatalf("invalid SetStartPhase = %v", err)
	}
	if err := lfo.SetRandomSeed(65535); err != nil || seed != 65535 {
		t.Fatalf("SetRandomSeed = %v, %d", err, seed)
	}
	if err := lfo.SetGlobal(true); err != nil || !lfoGlobal {
		t.Fatalf("LFO SetGlobal = %v, %v", err, lfoGlobal)
	}
	if err := lfo.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lfo.SetGlobal(false); !errors.Is(err, playdate.ErrAudioGraphClosed) {
		t.Fatalf("closed LFO SetGlobal = %v", err)
	}
}

func TestAudioChannelRoutesSynthSource(t *testing.T) {
	synth := NewSynth(4, SynthDriver{Audio: AudioDriver{Source: func(handle uintptr) uintptr { return handle }}})
	var routed uintptr
	adds := 0
	channel := NewAudioChannel(9, AudioChannelDriver{AddSource: func(_, source uintptr) bool { routed = source; adds++; return true }})
	if err := channel.AddSource(synth); err != nil || routed != 4 {
		t.Fatalf("AddSource(synth) = %v, routed %d", err, routed)
	}
	if err := channel.AddSource(synth); err != nil || adds != 2 {
		t.Fatalf("AddSource(synth) refresh = %v, calls %d", err, adds)
	}
}

func TestAudioChannelMovesSourceFromPreviousChannel(t *testing.T) {
	synth := NewSynth(4, SynthDriver{Audio: AudioDriver{Source: func(handle uintptr) uintptr { return handle }}})
	var removedFrom, addedTo uintptr
	driver := AudioChannelDriver{
		AddSource:    func(channel, _ uintptr) bool { addedTo = channel; return true },
		RemoveSource: func(channel, _ uintptr) bool { removedFrom = channel; return true },
	}
	first := NewAudioChannel(8, driver)
	second := NewAudioChannel(9, driver)
	if err := first.AddSource(synth); err != nil {
		t.Fatal(err)
	}
	if err := second.AddSource(synth); err != nil {
		t.Fatal(err)
	}
	if removedFrom != 8 || addedTo != 9 {
		t.Fatalf("move route removed from %d, added to %d", removedFrom, addedTo)
	}
	if err := first.RemoveSource(synth); err != nil || removedFrom != 8 {
		t.Fatalf("stale first route = %v, removed from %d", err, removedFrom)
	}
	if err := second.RemoveSource(synth); err != nil || removedFrom != 9 {
		t.Fatalf("active second route = %v, removed from %d", err, removedFrom)
	}
}

type musicContext struct{ testContext }

func TestAudioEffectGraphDetachesBothDirections(t *testing.T) {
	var added, removed uintptr
	channel := NewAudioChannel(9, AudioChannelDriver{
		AddEffect:    func(_, effect uintptr) bool { added = effect; return true },
		RemoveEffect: func(_, effect uintptr) bool { removed = effect; return true },
		Remove:       func(uintptr) bool { return true }, Free: func(uintptr) {},
	})
	var modulator uintptr
	freed := 0
	effect := NewRingModulator(7, RingModulatorDriver{
		Effect:       EffectDriver{SetMix: func(uintptr, float32) {}, SetMixModulator: func(uintptr, uintptr) {}, Free: func(uintptr) { freed++ }},
		SetFrequency: func(uintptr, float32) {}, SetFrequencyModulator: func(_, signal uintptr) { modulator = signal },
	})
	signal := NewLFO(3, LFODriver{Signal: SignalDriver{Value: func(uintptr) float32 { return 0 }, SetScale: func(uintptr, float32) {}, SetOffset: func(uintptr, float32) {}, Free: func(uintptr) {}}})
	if err := channel.AddEffect(effect); err != nil || added != 7 {
		t.Fatalf("AddEffect = %v, %d", err, added)
	}
	if err := effect.SetFrequencyModulator(signal); err != nil || modulator != 3 {
		t.Fatalf("SetFrequencyModulator = %v, %d", err, modulator)
	}
	if err := signal.Close(); err != nil || modulator != 0 {
		t.Fatalf("signal Close = %v, modulator %d", err, modulator)
	}
	if err := effect.Close(); err != nil || removed != 7 || freed != 1 {
		t.Fatalf("effect Close = %v, removed %d, freed %d", err, removed, freed)
	}
}

func TestAudioChannelRoutesDelayTap(t *testing.T) {
	var routed uintptr
	channel := NewAudioChannel(9, AudioChannelDriver{AddSource: func(_, source uintptr) bool { routed = source; return true }})
	delay := NewDelayLine(7, DelayLineDriver{AddTap: func(uintptr, int) uintptr { return 5 }, Tap: AudioDriver{Source: func(h uintptr) uintptr { return h }}})
	tap, err := delay.AddTap(100)
	if err != nil {
		t.Fatal(err)
	}
	if err = channel.AddSource(tap); err != nil || routed != 5 {
		t.Fatalf("AddSource(delay tap) = %v, routed %d", err, routed)
	}
}

func TestSequenceOwnershipAndCallbackLifetime(t *testing.T) {
	var trackInstrument, routedTrack uintptr
	var callback uint32
	trackCount := uint(0)
	freed := [3]int{}
	synth := NewSynth(1, SynthDriver{Audio: AudioDriver{Source: func(h uintptr) uintptr { return h }, Stop: func(uintptr) {}, Free: func(uintptr) { freed[0]++ }}}).(*synth)
	instrument := NewInstrument(2, InstrumentDriver{AddVoice: func(_, _ uintptr, _, _ uint8, _ float32) bool { return true }, Free: func(uintptr) { freed[1]++ }}).(*instrument)
	track := NewSequenceTrack(3, TrackDriver{SetInstrument: func(_, value uintptr) { trackInstrument = value }, Free: func(uintptr) { freed[2]++ }}).(*sequenceTrack)
	sequence := NewSequence(4, SequenceDriver{TrackCount: func(uintptr) uint { return trackCount }, AddTrack: func(uintptr) uintptr { trackCount++; return 99 }, SetTrack: func(_ uintptr, _ uint, value uintptr) { routedTrack = value }, Play: func(_ uintptr, value uint32) { callback = value }, Stop: func(uintptr) {}, Free: func(uintptr) {}}).(*sequence)
	if err := instrument.AddVoice(synth, 0, 127, 0); err != nil {
		t.Fatal(err)
	}
	if err := instrument.AddVoice(synth, 0, 127, 0); !errors.Is(err, playdate.ErrAudioRoute) {
		t.Fatalf("duplicate instrument voice = %v", err)
	}
	if err := NewAudioChannel(8, AudioChannelDriver{}).AddSource(synth); !errors.Is(err, playdate.ErrAudioRoute) {
		t.Fatalf("instrument synth channel route = %v", err)
	}
	if err := synth.Close(); !errors.Is(err, playdate.ErrAudioRoute) {
		t.Fatalf("attached synth Close = %v", err)
	}
	if err := track.SetInstrument(instrument); err != nil || trackInstrument != 2 {
		t.Fatalf("SetInstrument = %v, %d", err, trackInstrument)
	}
	if err := sequence.SetTrack(0, track); err != nil || routedTrack != 3 {
		t.Fatalf("SetTrack = %v, %d", err, routedTrack)
	}
	if trackCount != 1 {
		t.Fatalf("native track slots = %d", trackCount)
	}
	called := 0
	if err := sequence.Play(func() { called++ }); err != nil || callback == 0 {
		t.Fatalf("Play = %v, callback %d", err, callback)
	}
	InvokeAudioCallback(callback, true)
	DrainAudioCallbacks()
	if called != 1 {
		t.Fatalf("completion calls = %d", called)
	}
	if err := instrument.Close(); err != nil || trackInstrument != 0 {
		t.Fatalf("instrument Close = %v, track instrument %d", err, trackInstrument)
	}
	if err := synth.Close(); err != nil || freed[0] != 1 {
		t.Fatalf("detached synth Close = %v, frees %d", err, freed[0])
	}
	if err := track.Close(); err != nil || routedTrack != 0 || freed[2] != 1 {
		t.Fatalf("track Close = %v, sequence track %d, frees %d", err, routedTrack, freed[2])
	}
}

func TestInstrumentRejectsChannelRoutedSynth(t *testing.T) {
	synth := NewSynth(1, SynthDriver{Audio: AudioDriver{Source: func(h uintptr) uintptr { return h }}})
	channel := NewAudioChannel(2, AudioChannelDriver{AddSource: func(uintptr, uintptr) bool { return true }})
	if err := channel.AddSource(synth); err != nil {
		t.Fatal(err)
	}
	instrument := NewInstrument(3, InstrumentDriver{AddVoice: func(uintptr, uintptr, uint8, uint8, float32) bool { t.Fatal("native addVoice called"); return true }})
	if err := instrument.AddVoice(synth, 0, 127, 0); !errors.Is(err, playdate.ErrAudioRoute) {
		t.Fatalf("AddVoice routed synth = %v", err)
	}
}

func (musicContext) NewAudioChannel() (playdate.AudioChannel, error) {
	return NewAudioChannel(1, AudioChannelDriver{}), nil
}
func (musicContext) NewSynth(playdate.Waveform) (playdate.Synth, error) {
	return NewSynth(1, SynthDriver{}), nil
}
func (musicContext) NewLFO(playdate.LFOType) (playdate.LFO, error) {
	return NewLFO(1, LFODriver{}), nil
}
func (musicContext) NewEnvelope(float32, float32, float32, float32) (playdate.Envelope, error) {
	return NewEnvelope(1, EnvelopeDriver{}), nil
}
func (musicContext) NewControlSignal() (playdate.ControlSignal, error) {
	return NewControlSignal(1, ControlSignalDriver{}), nil
}
func (musicContext) NewInstrument() (playdate.Instrument, error)       { return nil, nil }
func (musicContext) NewSequenceTrack() (playdate.SequenceTrack, error) { return nil, nil }
func (musicContext) NewSequence() (playdate.Sequence, error)           { return nil, nil }
func (musicContext) NewTwoPoleFilter(playdate.FilterType) (playdate.TwoPoleFilter, error) {
	return nil, nil
}
func (musicContext) NewOnePoleFilter() (playdate.OnePoleFilter, error)  { return nil, nil }
func (musicContext) NewBitCrusher() (playdate.BitCrusher, error)        { return nil, nil }
func (musicContext) NewRingModulator() (playdate.RingModulator, error)  { return nil, nil }
func (musicContext) NewDelayLine(int, bool) (playdate.DelayLine, error) { return nil, nil }
func (musicContext) NewOverdrive() (playdate.Overdrive, error)          { return nil, nil }
func (musicContext) NewPCMCallbackSource(playdate.AudioChannel, bool, playdate.PCMRenderCallback) (playdate.PCMCallbackSource, error) {
	return nil, nil
}
func (musicContext) NewGeneratorSynth(bool, playdate.GeneratorRenderCallback) (playdate.GeneratorSynth, error) {
	return nil, nil
}

func TestApplicationForwardsMusicGraph(t *testing.T) {
	application, err := NewApplication(testGame{init: func(context playdate.Context) error {
		if _, ok := context.(playdate.AudioChannels); !ok {
			return errors.New("audio channels not forwarded")
		}
		if _, ok := context.(playdate.Synthesizers); !ok {
			return errors.New("synthesizers not forwarded")
		}
		if _, ok := context.(playdate.CallbackAudio); !ok {
			return errors.New("callback audio not forwarded")
		}
		if _, ok := context.(playdate.GeneratorSynthesizers); !ok {
			return errors.New("generator synths not forwarded")
		}
		sequencers, ok := context.(playdate.Sequencers)
		if !ok {
			return errors.New("sequencers not forwarded")
		}
		if _, err := sequencers.NewSequence(); err != nil {
			return err
		}
		effects, ok := context.(playdate.AudioEffects)
		if !ok {
			return errors.New("audio effects not forwarded")
		}
		if _, err := effects.NewOverdrive(); err != nil {
			return err
		}
		return nil
	}}, musicContext{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
}

func TestBitmapOwnershipLifecycle(t *testing.T) {
	var freed []uintptr
	var fills []playdate.Color
	driver := BitmapDriver{
		Dimensions: func(uintptr) (int, int) { return 400, 240 },
		Fill:       func(_ uintptr, color playdate.Color) { fills = append(fills, color) },
		Free:       func(handle uintptr) { freed = append(freed, handle) },
	}
	owned := NewOwnedBitmap(7, driver)
	if width, err := owned.Width(); err != nil || width != 400 {
		t.Fatalf("Width() = %d, %v", width, err)
	}
	if height, err := owned.Height(); err != nil || height != 240 {
		t.Fatalf("Height() = %d, %v", height, err)
	}
	if err := owned.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := owned.Fill(playdate.ColorBlack); err != nil {
		t.Fatal(err)
	}
	if err := owned.Fill(playdate.Color(99)); !errors.Is(err, playdate.ErrBitmapColor) {
		t.Fatalf("invalid Fill() error = %v", err)
	}
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	if len(freed) != 1 || freed[0] != 7 {
		t.Fatalf("freed = %v", freed)
	}
	if len(fills) != 2 || fills[0] != playdate.ColorClear || fills[1] != playdate.ColorBlack {
		t.Fatalf("fills = %v", fills)
	}
	for name, operation := range map[string]func() error{
		"double close":           owned.Close,
		"fill after close":       func() error { return owned.Fill(playdate.ColorWhite) },
		"dimensions after close": func() error { _, err := owned.Width(); return err },
	} {
		if err := operation(); !errors.Is(err, playdate.ErrBitmapClosed) {
			t.Fatalf("%s error = %v", name, err)
		}
	}
	borrowed := NewBorrowedBitmap(8, driver)
	if err := borrowed.Close(); !errors.Is(err, playdate.ErrBitmapBorrowed) {
		t.Fatalf("borrowed Close() = %v", err)
	}
	if _, err := BitmapHandle(borrowed); err != nil {
		t.Fatal(err)
	}
	if _, err := OwnedBitmapHandle(borrowed); !errors.Is(err, playdate.ErrBitmapBorrowed) {
		t.Fatalf("borrowed offscreen handle = %v", err)
	}
	owned = NewOwnedBitmap(9, driver)
	if handle, err := OwnedBitmapHandle(owned); err != nil || handle != 9 {
		t.Fatalf("owned offscreen handle = %d, %v", handle, err)
	}
}

func TestBitmapTableOwnershipAndBorrowedFrames(t *testing.T) {
	freed := uintptr(0)
	table := NewOwnedBitmapTable(11, BitmapTableDriver{Frame: func(_ uintptr, index int) uintptr {
		if index == 2 {
			return 22
		}
		return 0
	}, Free: func(handle uintptr) { freed = handle }}, BitmapDriver{})
	frame, err := table.Frame(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := frame.Close(); !errors.Is(err, playdate.ErrBitmapBorrowed) {
		t.Fatalf("frame close = %v", err)
	}
	if _, err := table.Frame(1); !errors.Is(err, playdate.ErrBitmapFrameRange) {
		t.Fatalf("range error = %v", err)
	}
	if err := table.Close(); err != nil {
		t.Fatal(err)
	}
	if freed != 11 {
		t.Fatalf("freed = %d", freed)
	}
	if _, err := frame.Width(); !errors.Is(err, playdate.ErrBitmapClosed) {
		t.Fatalf("frame after table close = %v", err)
	}
	if _, err := table.Frame(2); !errors.Is(err, playdate.ErrBitmapTableClosed) {
		t.Fatalf("closed error = %v", err)
	}
}

func TestOwnedBitmapDataLifetimePixelsAndMask(t *testing.T) {
	data := []byte{0, 0}
	mask := []byte{0xff, 0xff}
	driver := BitmapDriver{
		Dimensions: func(uintptr) (int, int) { return 9, 1 },
		Data:       func(uintptr) (int, int, int, []byte, []byte) { return 9, 1, 2, mask, data },
	}
	bitmap := NewOwnedBitmap(7, driver)
	var retained playdate.BitmapData
	if err := WithBitmapData(bitmap, func(view playdate.BitmapData) error {
		retained = view
		if view.Width() != 9 || view.Height() != 1 || view.RowBytes() != 2 {
			t.Fatalf("dimensions = %d x %d stride %d", view.Width(), view.Height(), view.RowBytes())
		}
		if bytes, err := view.MaskBytes(); err != nil || len(bytes) != 2 {
			t.Fatalf("mask = %v, %v", bytes, err)
		}
		if err := view.SetPixel(8, 0, playdate.ColorBlack); err != nil {
			return err
		}
		if dirty, err := view.Dirty(); err != nil || !dirty {
			t.Fatalf("dirty = %v, %v", dirty, err)
		}
		color, err := view.Pixel(8, 0)
		if err != nil || color != playdate.ColorBlack {
			t.Fatalf("pixel = %v, %v", color, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if data[1] != 0x80 {
		t.Fatalf("data = %x", data)
	}
	if _, err := retained.Bytes(); !errors.Is(err, playdate.ErrBitmapDataExpired) {
		t.Fatalf("retained bytes = %v", err)
	}
	if err := retained.SetPixel(0, 0, playdate.ColorBlack); !errors.Is(err, playdate.ErrBitmapDataExpired) {
		t.Fatalf("retained pixel = %v", err)
	}
	if err := WithBitmapData(NewBorrowedBitmap(8, driver), func(playdate.BitmapData) error { return nil }); !errors.Is(err, playdate.ErrBitmapBorrowed) {
		t.Fatalf("borrowed data = %v", err)
	}
	if err := WithBitmapData(bitmap, nil); !errors.Is(err, playdate.ErrBitmapDataCallback) {
		t.Fatalf("nil callback = %v", err)
	}
}

func TestBitmapMaskOwnershipAndLifetime(t *testing.T) {
	driver := BitmapDriver{Dimensions: func(uintptr) (int, int) { return 8, 8 }, Free: func(uintptr) {}}
	bitmap, mask := NewOwnedBitmap(1, driver), NewOwnedBitmap(2, driver)
	var nativeMask uintptr
	set := func(_ uintptr, value uintptr) bool { nativeMask = value; return true }
	if err := SetBitmapMask(bitmap, mask, set); err != nil {
		t.Fatal(err)
	}
	borrowed, ok, err := BitmapMask(bitmap, func(uintptr) uintptr { return nativeMask })
	if err != nil || !ok {
		t.Fatalf("mask = %v, %v", ok, err)
	}
	if err := bitmap.Close(); !errors.Is(err, playdate.ErrBitmapMaskInUse) {
		t.Fatalf("owner close with mask view = %v", err)
	}
	if err := borrowed.Close(); err != nil {
		t.Fatalf("mask view close = %v", err)
	}
	if err := bitmap.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := borrowed.Width(); !errors.Is(err, playdate.ErrBitmapClosed) {
		t.Fatalf("mask after owner close = %v", err)
	}
	wide := NewOwnedBitmap(3, BitmapDriver{Dimensions: func(uintptr) (int, int) { return 9, 8 }})
	if err := SetBitmapMask(wide, mask, set); !errors.Is(err, playdate.ErrBitmapMaskSize) {
		t.Fatalf("mask size = %v", err)
	}
}

func TestBitmapArgumentValidation(t *testing.T) {
	if err := ValidateBitmapSize(0, 1); !errors.Is(err, playdate.ErrBitmapSize) {
		t.Fatalf("size error = %v", err)
	}
	if err := ValidateBitmapScale(1, 0); !errors.Is(err, playdate.ErrBitmapScale) {
		t.Fatalf("scale error = %v", err)
	}
	if err := ValidateBitmapScale(1, *(*float32)(unsafe.Pointer(&[]uint32{0x7fc00000}[0]))); !errors.Is(err, playdate.ErrBitmapScale) {
		t.Fatalf("NaN scale error = %v", err)
	}
	if err := ValidateBitmapTransform(*(*float32)(unsafe.Pointer(&[]uint32{0x7f800000}[0])), .5, .5, 1, 1); !errors.Is(err, playdate.ErrGraphicsGeometry) {
		t.Fatalf("infinite rotation error = %v", err)
	}
	narrow := NewOwnedBitmap(7, BitmapDriver{Dimensions: func(uintptr) (int, int) { return 31, 8 }})
	if _, err := ValidateStencil(narrow, true); !errors.Is(err, playdate.ErrGraphicsStencilWidth) {
		t.Fatalf("tiled stencil width error = %v", err)
	}
	wide := NewOwnedBitmap(8, BitmapDriver{Dimensions: func(uintptr) (int, int) { return 32, 8 }})
	if handle, err := ValidateStencil(wide, true); err != nil || handle != 8 {
		t.Fatalf("valid tiled stencil = %d, %v", handle, err)
	}
}

func TestPrimitiveAndDrawModeValidation(t *testing.T) {
	if err := ValidatePrimitiveGeometry(0, 1, 1, 0, 360); !errors.Is(err, playdate.ErrGraphicsGeometry) {
		t.Fatalf("width error = %v", err)
	}
	if err := ValidatePrimitiveGeometry(1, 1, 1, 0, 360); err != nil {
		t.Fatalf("valid geometry = %v", err)
	}
	if err := ValidateDrawMode(playdate.DrawMode(99)); !errors.Is(err, playdate.ErrGraphicsDrawMode) {
		t.Fatalf("draw mode error = %v", err)
	}
	if err := ValidateLineCapStyle(playdate.LineCapStyle(99)); !errors.Is(err, playdate.ErrGraphicsLineCap) {
		t.Fatalf("line cap error = %v", err)
	}
	if err := ValidatePolygon([]playdate.GraphicsPoint{{X: 0, Y: 0}, {X: 1, Y: 1}}, playdate.PolygonFillNonZero); !errors.Is(err, playdate.ErrGraphicsPolygon) {
		t.Fatalf("short polygon error = %v", err)
	}
	if err := ValidatePolygon([]playdate.GraphicsPoint{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}}, playdate.PolygonFillRule(99)); !errors.Is(err, playdate.ErrGraphicsPolygon) {
		t.Fatalf("fill rule error = %v", err)
	}
}

func TestSpriteDisplayListOwnershipLifecycle(t *testing.T) {
	var operations []string
	driver := SpriteDriver{
		SetBitmap:  func(sprite, bitmap uintptr) { operations = append(operations, "bitmap") },
		MoveTo:     func(uintptr, float32, float32) { operations = append(operations, "position") },
		MoveBy:     func(uintptr, float32, float32) { operations = append(operations, "move") },
		SetVisible: func(uintptr, bool) { operations = append(operations, "visible") },
		SetZIndex:  func(uintptr, int) { operations = append(operations, "z") },
		MarkDirty:  func(uintptr) { operations = append(operations, "dirty") },
		MarkDirtyRect: func(uintptr, playdate.Rect) {
			operations = append(operations, "dirtyRect")
		},
		Add:    func(uintptr) { operations = append(operations, "add") },
		Remove: func(uintptr) { operations = append(operations, "remove") },
		Free:   func(uintptr) { operations = append(operations, "free") },
	}
	sprite := NewOwnedSprite(9, driver)
	bitmap := NewOwnedBitmap(7, BitmapDriver{})
	if err := sprite.SetBitmap(bitmap); err != nil {
		t.Fatal(err)
	}
	if err := sprite.SetPosition(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := sprite.MoveBy(3, 4); err != nil {
		t.Fatal(err)
	}
	if err := sprite.SetVisible(true); err != nil {
		t.Fatal(err)
	}
	if err := sprite.SetZIndex(5); err != nil {
		t.Fatal(err)
	}
	if err := sprite.MarkDirty(); err != nil {
		t.Fatal(err)
	}
	if err := sprite.MarkDirtyRect(playdate.Rect{Width: 2, Height: 3}); err != nil {
		t.Fatal(err)
	}
	if err := sprite.Add(); err != nil {
		t.Fatal(err)
	}
	if err := sprite.Add(); err != nil {
		t.Fatal(err)
	}
	if err := sprite.Close(); err != nil {
		t.Fatal(err)
	}
	want := []string{"bitmap", "position", "move", "visible", "z", "dirty", "dirtyRect", "add", "remove", "free"}
	if len(operations) != len(want) {
		t.Fatalf("operations = %v, want %v", operations, want)
	}
	for index := range want {
		if operations[index] != want[index] {
			t.Fatalf("operations = %v, want %v", operations, want)
		}
	}
	if err := sprite.Close(); !errors.Is(err, playdate.ErrSpriteClosed) {
		t.Fatalf("double Close() = %v", err)
	}
}

func TestSpriteCallbacksDispatchAndCleanup(t *testing.T) {
	state := NewSpriteDisplayListState()
	var draw, update int
	driver := SpriteDriver{DisplayList: state, SetDrawCallback: func(uintptr, bool) {}, SetUpdateCallback: func(uintptr, bool) {}, SetCollisionCallback: func(uintptr, bool) {}, Free: func(uintptr) {}}
	sprite := NewOwnedSprite(9, driver)
	if err := sprite.SetDrawCallback(func(got playdate.Sprite, bounds, dirty playdate.Rect) {
		draw++
		if got != sprite || bounds.Width != 4 || dirty.Height != 2 {
			t.Fatal("draw callback arguments")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := sprite.SetUpdateCallback(func(got playdate.Sprite) {
		update++
		if got != sprite {
			t.Fatal("update sprite")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := sprite.SetCollisionResponseCallback(func(got, other playdate.Sprite) playdate.CollisionResponse {
		if got != sprite {
			t.Fatal("collision sprite")
		}
		if h, _ := SpriteHandle(other); h != 10 {
			t.Fatal("collision other")
		}
		return playdate.CollisionBounce
	}); err != nil {
		t.Fatal(err)
	}
	state.InvokeSpriteDraw(9, playdate.Rect{Width: 4}, playdate.Rect{Height: 2})
	state.InvokeSpriteUpdate(9)
	if got := state.InvokeSpriteCollision(9, 10); got != playdate.CollisionBounce {
		t.Fatalf("response = %v", got)
	}
	if draw != 1 || update != 1 {
		t.Fatalf("callbacks = %d/%d", draw, update)
	}
	state.TerminateSpriteCallbacks()
	state.InvokeSpriteDraw(9, playdate.Rect{}, playdate.Rect{})
	state.InvokeSpriteUpdate(9)
	if draw != 1 || update != 1 {
		t.Fatal("callback delivered after termination")
	}
	if err := sprite.Close(); err != nil {
		t.Fatal(err)
	}
	if state.callbacks != 0 || len(state.byHandle) != 0 {
		t.Fatalf("callback cleanup = %d/%d", state.callbacks, len(state.byHandle))
	}
}

func TestSpriteCallbackRegistryBound(t *testing.T) {
	state := NewSpriteDisplayListState()
	driver := SpriteDriver{DisplayList: state, SetUpdateCallback: func(uintptr, bool) {}, Free: func(uintptr) {}}
	for i := 0; i < spriteCallbackLimit; i++ {
		s := NewOwnedSprite(uintptr(i+1), driver)
		if err := s.SetUpdateCallback(func(playdate.Sprite) {}); err != nil {
			t.Fatal(err)
		}
	}
	extra := NewOwnedSprite(100, driver)
	if err := extra.SetUpdateCallback(func(playdate.Sprite) {}); !errors.Is(err, playdate.ErrSpriteCallbackLimit) {
		t.Fatalf("limit error = %v", err)
	}
}

func TestSpriteGeometryPresentationAndEnableState(t *testing.T) {
	driver := SpriteDriver{
		SetCenter: func(uintptr, float32, float32) {}, Center: func(uintptr) (float32, float32) { return .25, .75 },
		SetBounds: func(uintptr, playdate.Rect) {}, Bounds: func(uintptr) playdate.Rect { return playdate.Rect{X: 1, Y: 2, Width: 3, Height: 4} },
		Position: func(uintptr) (float32, float32) { return 5, 6 }, Visible: func(uintptr) bool { return true }, ZIndex: func(uintptr) int { return -2 },
		SetImageFlip: func(uintptr, playdate.BitmapFlip) {}, ImageFlip: func(uintptr) playdate.BitmapFlip { return playdate.BitmapFlippedX },
		SetDrawMode: func(uintptr, playdate.DrawMode) {}, SetOpaque: func(uintptr, bool) {}, SetStencilImage: func(uintptr, uintptr, bool) {}, SetStencilPattern: func(uintptr, [8]byte) {}, ClearStencil: func(uintptr) {},
		SetClipRect: func(uintptr, int, int, int, int) {}, ClearClipRect: func(uintptr) {}, SetIgnoresDrawOffset: func(uintptr, bool) {},
		SetUpdatesEnabled: func(uintptr, bool) {}, UpdatesEnabled: func(uintptr) bool { return false }, SetCollisionsEnabled: func(uintptr, bool) {}, CollisionsEnabled: func(uintptr) bool { return true },
		CollideRect: func(uintptr) playdate.Rect { return playdate.Rect{Width: 7, Height: 8} }, Tag: func(uintptr) uint8 { return 9 },
		Free: func(uintptr) {},
	}
	s := NewOwnedSprite(1, driver)
	b := NewOwnedBitmap(2, BitmapDriver{})
	if err := s.SetCenter(.25, .75); err != nil {
		t.Fatal(err)
	}
	if x, y, err := s.Center(); err != nil || x != .25 || y != .75 {
		t.Fatalf("center = %v,%v,%v", x, y, err)
	}
	if err := s.SetBounds(playdate.Rect{Width: 3, Height: 4}); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Bounds(); err != nil || r.Width != 3 || r.Height != 4 {
		t.Fatalf("bounds = %#v,%v", r, err)
	}
	if err := s.SetImageFlip(playdate.BitmapFlippedX); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDrawMode(playdate.DrawModeXOR); err != nil {
		t.Fatal(err)
	}
	if err := s.SetOpaque(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStencilImage(b, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStencilPattern([8]byte{0xaa}); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearStencil(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetClipRect(1, 2, 3, 4); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIgnoresDrawOffset(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUpdatesEnabled(false); err != nil {
		t.Fatal(err)
	}
	if enabled, err := s.UpdatesEnabled(); err != nil || enabled {
		t.Fatalf("updates = %v,%v", enabled, err)
	}
	if err := s.SetCollisionsEnabled(true); err != nil {
		t.Fatal(err)
	}
	if enabled, err := s.CollisionsEnabled(); err != nil || !enabled {
		t.Fatalf("collisions = %v,%v", enabled, err)
	}
	if err := s.SetClipRect(0, 0, 0, 1); !errors.Is(err, playdate.ErrSpriteDirtyRect) {
		t.Fatalf("clip error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Center(); !errors.Is(err, playdate.ErrSpriteClosed) {
		t.Fatalf("closed getter = %v", err)
	}
}

func TestSpriteTileMapOwnershipAttachmentAndTiles(t *testing.T) {
	var nativeTile uint16 = 2
	var attached uintptr
	var freed bool
	tileDriver := SpriteTileMapDriver{
		SetImageTable: func(uintptr, uintptr) {}, SetSize: func(uintptr, int, int) {},
		Size: func(uintptr) (int, int) { return 2, 2 }, PixelSize: func(uintptr) (int, int) { return 32, 32 },
		SetTiles: func(_ uintptr, tiles []uint16, row int) {
			if len(tiles) != 4 || row != 2 {
				t.Fatalf("tiles = %v row %d", tiles, row)
			}
		},
		SetTile: func(_ uintptr, _, _ int, value uint16) { nativeTile = value }, Tile: func(uintptr, int, int) uint16 { return nativeTile },
		Free: func(uintptr) { freed = true },
	}
	table := NewOwnedBitmapTable(4, BitmapTableDriver{Free: func(uintptr) {}}, BitmapDriver{})
	tileMap, err := NewOwnedSpriteTileMap(7, table, 2, 2, []uint16{1, 2, 2, 1}, tileDriver)
	if err != nil {
		t.Fatal(err)
	}
	if err := table.Close(); !errors.Is(err, playdate.ErrBitmapTableInUse) {
		t.Fatalf("table Close = %v", err)
	}
	if w, h, err := tileMap.Size(); err != nil || w != 2 || h != 2 {
		t.Fatalf("size = %d,%d,%v", w, h, err)
	}
	if w, h, err := tileMap.PixelSize(); err != nil || w != 32 || h != 32 {
		t.Fatalf("pixels = %d,%d,%v", w, h, err)
	}
	if err := tileMap.SetTile(1, 1, 5); err != nil {
		t.Fatal(err)
	}
	if value, err := tileMap.Tile(1, 1); err != nil || value != 5 {
		t.Fatalf("tile = %d,%v", value, err)
	}
	if _, err := tileMap.Tile(2, 0); !errors.Is(err, playdate.ErrSpriteTileMapBounds) {
		t.Fatalf("bounds = %v", err)
	}
	spriteDriver := SpriteDriver{SetTileMap: func(_ uintptr, m uintptr) { attached = m }, TileMap: func(uintptr) uintptr { return attached }, TileMaps: tileDriver, Free: func(uintptr) {}}
	sprite := NewOwnedSprite(9, spriteDriver)
	if err := sprite.SetTileMap(tileMap); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := sprite.TileMap(); err != nil || !ok || got != tileMap {
		t.Fatalf("TileMap = %v,%v,%v", got, ok, err)
	}
	if err := tileMap.Close(); !errors.Is(err, playdate.ErrSpriteTileMapInUse) {
		t.Fatalf("attached Close = %v", err)
	}
	if err := sprite.ClearTileMap(); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := sprite.TileMap(); err != nil || ok || got != nil {
		t.Fatalf("cleared TileMap = %v,%v,%v", got, ok, err)
	}
	if err := tileMap.Close(); err != nil {
		t.Fatal(err)
	}
	if !freed {
		t.Fatal("native tilemap was not freed")
	}
	if err := table.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sprite.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDisplayAndDirtyRegionValidation(t *testing.T) {
	var displayCalls int
	display := NewDisplay(DisplayDriver{
		Width: func() int { return 200 }, Height: func() int { return 120 },
		RefreshRate: func() float32 { return 30 }, FPS: func() float32 { return 29.5 },
		SetRefreshRate: func(float32) { displayCalls++ }, SetInverted: func(bool) { displayCalls++ },
		SetScale: func(uint) { displayCalls++ }, SetMosaic: func(uint, uint) { displayCalls++ },
		SetFlipped: func(bool, bool) { displayCalls++ }, SetOffset: func(int, int) { displayCalls++ },
	})
	if width, height := display.Width(), display.Height(); width != 200 || height != 120 {
		t.Fatalf("display size = %dx%d", width, height)
	}
	if rate, fps := display.RefreshRate(), display.FPS(); rate != 30 || fps != 29.5 {
		t.Fatalf("display rates = %v/%v", rate, fps)
	}
	if err := display.SetRefreshRate(50); err != nil {
		t.Fatal(err)
	}
	if err := display.SetScale(4); err != nil {
		t.Fatal(err)
	}
	if err := display.SetMosaic(3, 0); err != nil {
		t.Fatal(err)
	}
	display.SetInverted(true)
	display.SetFlipped(true, false)
	display.SetOffset(1, -1)
	if displayCalls != 6 {
		t.Fatalf("display calls = %d", displayCalls)
	}
	if err := display.SetRefreshRate(51); !errors.Is(err, playdate.ErrDisplayRefreshRate) {
		t.Fatalf("refresh error = %v", err)
	}
	if err := display.SetScale(3); !errors.Is(err, playdate.ErrDisplayScale) {
		t.Fatalf("scale error = %v", err)
	}
	if err := display.SetMosaic(0, 4); !errors.Is(err, playdate.ErrDisplayMosaic) {
		t.Fatalf("mosaic error = %v", err)
	}
	if err := ValidateScreenDirtyRect(0, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScreenDirtyRect(0, 0, 0, 1); !errors.Is(err, playdate.ErrSpriteDirtyRect) {
		t.Fatalf("screen rect error = %v", err)
	}
	if err := ValidateSpriteRect(playdate.Rect{Width: 1, Height: 1}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSpriteRect(playdate.Rect{Width: -1, Height: 1}); !errors.Is(err, playdate.ErrSpriteDirtyRect) {
		t.Fatalf("sprite rect error = %v", err)
	}
}

func TestVideoPlayerOwnershipValidationAndErrors(t *testing.T) {
	var freed int
	bitmap := NewOwnedBitmap(7, BitmapDriver{Dimensions: func(uintptr) (int, int) { return 80, 60 }, Free: func(uintptr) {}})
	player := NewVideoPlayer(9, VideoDriver{
		Info: func(uintptr) playdate.VideoInfo {
			return playdate.VideoInfo{Width: 80, Height: 60, FrameRate: 20, FrameCount: 3}
		},
		Error:            func(uintptr) string { return "decoder status" },
		SetContext:       func(player, context uintptr) (bool, string) { return player == 9 && context == 7, "bad context" },
		UseScreenContext: func(uintptr) {},
		RenderFrame:      func(_ uintptr, frame int) (bool, string) { return frame != 1, "decode failed" },
		Free:             func(uintptr) { freed++ },
	})
	if info, err := player.Info(); err != nil || info.FrameCount != 3 {
		t.Fatalf("Info() = %+v, %v", info, err)
	}
	if message, err := player.LastError(); err != nil || message != "decoder status" {
		t.Fatalf("LastError() = %q, %v", message, err)
	}
	if err := player.SetContext(bitmap); err != nil {
		t.Fatal(err)
	}
	if err := player.RenderFrame(-1); !errors.Is(err, playdate.ErrVideoFrame) {
		t.Fatalf("negative frame error = %v", err)
	}
	if err := player.RenderFrame(1); err == nil || err.Error() != "render frame video failed: decode failed" {
		t.Fatalf("decoder error = %v", err)
	}
	if err := bitmap.Close(); err != nil {
		t.Fatal(err)
	}
	if err := player.RenderFrame(0); !errors.Is(err, playdate.ErrBitmapClosed) {
		t.Fatalf("closed context error = %v", err)
	}
	if err := player.UseScreenContext(); err != nil {
		t.Fatal(err)
	}
	if err := player.RenderFrame(0); err != nil {
		t.Fatal(err)
	}
	if err := player.Close(); err != nil || freed != 1 {
		t.Fatalf("Close() = %v, frees %d", err, freed)
	}
	if err := player.Close(); !errors.Is(err, playdate.ErrVideoClosed) {
		t.Fatalf("second Close() = %v", err)
	}
}

type videoContext struct {
	testContext
	player playdate.VideoPlayer
}

func (c videoContext) LoadVideo(path string) (playdate.VideoPlayer, error) {
	if path != "intro.pdv" {
		return nil, errors.New("wrong path")
	}
	return c.player, nil
}

type videoCapabilityGame struct{ got playdate.VideoPlayer }

func (g *videoCapabilityGame) Init(context playdate.Context) error {
	player, err := context.(playdate.Videos).LoadVideo("intro.pdv")
	g.got = player
	return err
}
func (*videoCapabilityGame) Update(playdate.Context) (bool, error) { return false, nil }

func TestNewApplicationForwardsVideoCapability(t *testing.T) {
	player := NewVideoPlayer(1, VideoDriver{})
	game := &videoCapabilityGame{}
	application, err := NewApplication(game, videoContext{player: player}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if game.got != player {
		t.Fatal("video player was not forwarded")
	}
}

type displayCapabilityContext struct {
	testContext
	calls int
}

func (*displayCapabilityContext) Width() int                              { return 200 }
func (*displayCapabilityContext) Height() int                             { return 120 }
func (*displayCapabilityContext) RefreshRate() float32                    { return 30 }
func (*displayCapabilityContext) FPS() float32                            { return 29 }
func (c *displayCapabilityContext) SetRefreshRate(float32) error          { c.calls++; return nil }
func (*displayCapabilityContext) SetInverted(bool)                        {}
func (*displayCapabilityContext) SetScale(uint) error                     { return nil }
func (*displayCapabilityContext) SetMosaic(uint, uint) error              { return nil }
func (*displayCapabilityContext) SetFlipped(bool, bool)                   {}
func (*displayCapabilityContext) SetOffset(int, int)                      {}
func (c *displayCapabilityContext) SetAlwaysRedraw(bool)                  { c.calls++ }
func (c *displayCapabilityContext) AddDirtyRect(int, int, int, int) error { c.calls++; return nil }
func (c *displayCapabilityContext) QuerySpritesAlongLine(float32, float32, float32, float32) []playdate.Sprite {
	c.calls++
	return nil
}
func (c *displayCapabilityContext) QuerySpriteInfoAlongLine(float32, float32, float32, float32) []playdate.SpriteQueryInfo {
	c.calls++
	return nil
}
func (c *displayCapabilityContext) SpriteCount() int                      { c.calls++; return 2 }
func (c *displayCapabilityContext) AddSprites([]playdate.Sprite) error    { c.calls++; return nil }
func (c *displayCapabilityContext) RemoveSprites([]playdate.Sprite) error { c.calls++; return nil }
func (c *displayCapabilityContext) RemoveAllSprites()                     { c.calls++ }
func (c *displayCapabilityContext) ResetCollisionWorld()                  { c.calls++ }

type displayCapabilityGame struct{}

func (displayCapabilityGame) Init(context playdate.Context) error {
	display := context.(playdate.Display)
	if display.Width() != 200 || display.Height() != 120 || display.RefreshRate() != 30 || display.FPS() != 29 {
		return errors.New("display introspection was not forwarded")
	}
	if err := display.SetRefreshRate(30); err != nil {
		return err
	}
	context.(playdate.SpriteRedraw).SetAlwaysRedraw(false)
	if err := context.(playdate.SpriteRedraw).AddDirtyRect(1, 2, 3, 4); err != nil {
		return err
	}
	queries := context.(playdate.SpriteQueries)
	queries.QuerySpritesAlongLine(0, 0, 1, 1)
	queries.QuerySpriteInfoAlongLine(0, 0, 1, 1)
	list := context.(playdate.SpriteDisplayList)
	if list.SpriteCount() != 2 {
		return errors.New("sprite count was not forwarded")
	}
	if err := list.AddSprites(nil); err != nil {
		return err
	}
	if err := list.RemoveSprites(nil); err != nil {
		return err
	}
	list.RemoveAllSprites()
	list.ResetCollisionWorld()
	return nil
}
func (displayCapabilityGame) Update(playdate.Context) (bool, error) { return false, nil }

func TestApplicationForwardsDisplayAndSpriteRedrawCapabilities(t *testing.T) {
	context := &displayCapabilityContext{}
	application, err := NewApplication(displayCapabilityGame{}, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if context.calls != 10 {
		t.Fatalf("forwarded calls = %d", context.calls)
	}
}

func TestSpriteCollisionBridgeAndBorrowedResponse(t *testing.T) {
	var operations []string
	driver := SpriteDriver{
		SetCollideRect:   func(uintptr, playdate.Rect) { operations = append(operations, "rect") },
		ClearCollideRect: func(uintptr) { operations = append(operations, "clear") },
		SetTag:           func(uintptr, uint8) { operations = append(operations, "tag") },
		MoveWithCollisions: func(uintptr, float32, float32) (float32, float32, []NativeCollision) {
			return 8, 9, []NativeCollision{{Other: 12, ResponseType: playdate.CollisionOverlap, Time: .5}}
		},
		CheckCollisions: func(uintptr, float32, float32) (float32, float32, []NativeCollision) {
			return 10, 11, []NativeCollision{{Other: 13, ResponseType: playdate.CollisionBounce}}
		},
	}
	sprite := NewOwnedSprite(11, driver)
	if err := sprite.SetCollideRect(playdate.Rect{Width: 16, Height: 16}); err != nil {
		t.Fatal(err)
	}
	if err := sprite.SetTag(2); err != nil {
		t.Fatal(err)
	}
	result, err := sprite.MoveWithCollisions(10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActualX != 8 || result.ActualY != 9 || len(result.Collisions) != 1 || result.Collisions[0].ResponseType != playdate.CollisionOverlap {
		t.Fatalf("result = %+v", result)
	}
	if err := result.Collisions[0].Other.Close(); !errors.Is(err, playdate.ErrSpriteBorrowed) {
		t.Fatalf("borrowed Close() = %v", err)
	}
	checked, err := sprite.CheckCollisions(12, 13)
	if err != nil || checked.ActualX != 10 || checked.ActualY != 11 || len(checked.Collisions) != 1 || checked.Collisions[0].ResponseType != playdate.CollisionBounce {
		t.Fatalf("CheckCollisions() = %+v, %v", checked, err)
	}
	if err := sprite.ClearCollideRect(); err != nil {
		t.Fatal(err)
	}
	want := []string{"rect", "tag", "clear"}
	if len(operations) != len(want) {
		t.Fatalf("operations = %v", operations)
	}
	for index := range want {
		if operations[index] != want[index] {
			t.Fatalf("operations = %v", operations)
		}
	}
}

func TestSpriteBatchValidationAndRemoveAllState(t *testing.T) {
	state := NewSpriteDisplayListState()
	removeCalls := 0
	driver := SpriteDriver{DisplayList: state, Add: func(uintptr) {}, Remove: func(uintptr) { removeCalls++ }, Free: func(uintptr) {}}
	first := NewOwnedSprite(1, driver)
	second := NewOwnedSprite(2, driver)
	if err := SetSpritesAdded([]playdate.Sprite{first, second}, true, func(handles []uintptr) {
		if len(handles) != 2 || handles[0] != 1 || handles[1] != 2 {
			t.Fatalf("handles = %v", handles)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetSpritesAdded([]playdate.Sprite{first, second}, true, func(handles []uintptr) {
		if len(handles) != 0 {
			t.Fatalf("idempotent handles = %v", handles)
		}
	}); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := SetSpritesAdded([]playdate.Sprite{first, nil}, false, func([]uintptr) { called = true }); !errors.Is(err, playdate.ErrSpriteClosed) {
		t.Fatalf("invalid batch error = %v", err)
	}
	if called {
		t.Fatal("partially executed invalid batch")
	}
	state.RemoveAll()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if removeCalls != 0 {
		t.Fatalf("remove calls after remove-all = %d", removeCalls)
	}
}

type testGame struct {
	init   func(playdate.Context) error
	update func(playdate.Context) (bool, error)
}

func (g testGame) Init(context playdate.Context) error { return g.init(context) }
func (g testGame) Update(context playdate.Context) (bool, error) {
	return g.update(context)
}

func TestABIFixedWidthTypes(t *testing.T) {
	if got := unsafe.Sizeof(Event(0)); got != 4 {
		t.Fatalf("sizeof(Event) = %d, want 4", got)
	}
}

func TestNewRequiresCallbacks(t *testing.T) {
	if _, err := New(Callbacks{}); !errors.Is(err, ErrInitRequired) {
		t.Fatalf("New() error = %v, want ErrInitRequired", err)
	}
	if _, err := New(Callbacks{Init: func() error { return nil }}); !errors.Is(err, ErrUpdateRequired) {
		t.Fatalf("New() error = %v, want ErrUpdateRequired", err)
	}
}

func TestRuntimeLifecycle(t *testing.T) {
	initCalls := 0
	updateCalls := 0
	runtime, err := New(Callbacks{
		Init: func() error {
			initCalls++
			return nil
		},
		Update: func(playdate.Input) (bool, error) {
			updateCalls++
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Update(RawInput{}); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Update() before init error = %v, want ErrNotInitialized", err)
	}
	if err := runtime.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	refresh, err := runtime.Update(RawInput{})
	if err != nil {
		t.Fatal(err)
	}
	if refresh != 1 || initCalls != 1 || updateCalls != 1 {
		t.Fatalf("refresh/init/update = %d/%d/%d, want 1/1/1", refresh, initCalls, updateCalls)
	}
	if err := runtime.Handle(EventInit, 0); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("duplicate Handle(EventInit) error = %v, want ErrAlreadyInitialized", err)
	}
}

func TestRuntimeRefreshAndErrors(t *testing.T) {
	updateErr := errors.New("update failed")
	tests := []struct {
		name        string
		update      func(playdate.Input) (bool, error)
		wantRefresh int32
		wantErr     error
	}{
		{"no refresh", func(playdate.Input) (bool, error) { return false, nil }, 0, nil},
		{"update error", func(playdate.Input) (bool, error) { return true, updateErr }, 0, updateErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime, err := New(Callbacks{Init: func() error { return nil }, Update: test.update})
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.Handle(EventInit, 0); err != nil {
				t.Fatal(err)
			}
			refresh, err := runtime.Update(RawInput{})
			if refresh != test.wantRefresh || !errors.Is(err, test.wantErr) {
				t.Fatalf("Update() = %d, %v; want %d, %v", refresh, err, test.wantRefresh, test.wantErr)
			}
		})
	}
}

func TestRuntimeFailedInitializationCannotUpdate(t *testing.T) {
	initErr := errors.New("init failed")
	runtime, err := New(Callbacks{
		Init:   func() error { return initErr },
		Update: func(playdate.Input) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Handle(EventInit, 0); !errors.Is(err, initErr) {
		t.Fatalf("Handle() error = %v, want init error", err)
	}
	if _, err := runtime.Update(RawInput{}); !errors.Is(err, ErrFailed) {
		t.Fatalf("Update() error = %v, want ErrFailed", err)
	}
}

func TestApplicationIsCommonLifecycleEntryPoint(t *testing.T) {
	var calls []string
	context := &launcherContext{}
	application, err := NewApplication(testGame{
		init: func(got playdate.Context) error {
			calls = append(calls, "init")
			fonts, ok := got.(playdate.FontGraphics)
			if !ok {
				return errors.New("font graphics not forwarded")
			}
			if _, fontErr := fonts.LoadFont("fonts/test"); fontErr != nil {
				return fontErr
			}
			launcher, ok := got.(playdate.Launcher)
			if !ok {
				return errors.New("launcher not forwarded")
			}
			launcher.ExitToLauncher()
			return nil
		},
		update: func(got playdate.Context) (bool, error) {
			calls = append(calls, "update")
			return true, nil
		},
	}, context, func() { calls = append(calls, "before-init") })
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	refresh, err := application.Update(RawInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := refresh, int32(1); got != want {
		t.Fatalf("Update() refresh = %d, want %d", got, want)
	}
	wantCalls := []string{"before-init", "init", "update"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	for index := range wantCalls {
		if calls[index] != wantCalls[index] {
			t.Fatalf("calls = %v, want %v", calls, wantCalls)
		}
	}
	if !context.exited {
		t.Fatal("ExitToLauncher was not forwarded")
	}
}

func TestLifecycleEventsPreservePlatformOrder(t *testing.T) {
	var got []playdate.LifecycleEvent
	runtime, err := New(Callbacks{
		Init: func() error { return nil },
		Lifecycle: func(event playdate.LifecycleEvent) error {
			got = append(got, event)
			return nil
		},
		Update: func(playdate.Input) (bool, error) { return false, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	events := []Event{EventPause, EventLock, EventLowPower, EventUnlock, EventResume, EventTerminate}
	for _, event := range events {
		if err := runtime.Handle(event, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := runtime.Update(RawInput{}); !errors.Is(err, ErrTerminated) {
		t.Fatalf("Update() after terminate error = %v, want ErrTerminated", err)
	}
	want := []playdate.LifecycleEvent{playdate.LifecyclePause, playdate.LifecycleLock, playdate.LifecycleLowPower, playdate.LifecycleUnlock, playdate.LifecycleResume, playdate.LifecycleTerminate}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("events = %v, want %v", got, want)
		}
	}
}

func TestLifecycleErrorIsFailStop(t *testing.T) {
	wantErr := errors.New("pause failed")
	runtime, err := New(Callbacks{
		Init:      func() error { return nil },
		Lifecycle: func(playdate.LifecycleEvent) error { return wantErr },
		Update:    func(playdate.Input) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Handle(EventPause, 0); !errors.Is(err, wantErr) {
		t.Fatalf("pause error = %v, want %v", err, wantErr)
	}
	if _, err := runtime.Update(RawInput{}); !errors.Is(err, ErrFailed) {
		t.Fatalf("Update() error = %v, want ErrFailed", err)
	}
}

func TestInputTransitionsBetweenFrames(t *testing.T) {
	var snapshots []playdate.Input
	runtime, err := New(Callbacks{
		Init: func() error { return nil },
		Update: func(input playdate.Input) (bool, error) {
			snapshots = append(snapshots, input)
			return false, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	frames := []RawInput{
		{Buttons: playdate.ButtonA, CrankAngle: 10, CrankDelta: 2, CrankDocked: true, DeltaSeconds: 0.016},
		{Buttons: playdate.ButtonA | playdate.ButtonLeft, CrankAngle: 15, CrankDelta: 5, CrankDocked: false, DeltaSeconds: 0.017},
		{Buttons: playdate.ButtonLeft, CrankAngle: 15, CrankDocked: true, DeltaSeconds: 0.018},
	}
	for _, frame := range frames {
		if _, err := runtime.Update(frame); err != nil {
			t.Fatal(err)
		}
	}
	if got := snapshots[0]; got.Pressed != playdate.ButtonA || got.Held != 0 || got.Released != 0 || got.CrankDockedThisFrame || got.CrankUndocked {
		t.Fatalf("first snapshot transitions = %+v", got)
	}
	if got := snapshots[1]; got.Pressed != playdate.ButtonLeft || got.Held != playdate.ButtonA || got.Released != 0 || !got.CrankUndocked || got.DeltaSeconds != 0.017 {
		t.Fatalf("second snapshot transitions = %+v", got)
	}
	if got := snapshots[2]; got.Pressed != 0 || got.Held != playdate.ButtonLeft || got.Released != playdate.ButtonA || !got.CrankDockedThisFrame {
		t.Fatalf("third snapshot transitions = %+v", got)
	}
}
