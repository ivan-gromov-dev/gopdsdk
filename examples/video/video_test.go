package video

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type bitmap struct{ closed bool }

func (*bitmap) Width() (int, error)       { return 160, nil }
func (*bitmap) Height() (int, error)      { return 96, nil }
func (*bitmap) Clear() error              { return nil }
func (*bitmap) Fill(playdate.Color) error { return nil }
func (b *bitmap) Close() error            { b.closed = true; return nil }

type player struct {
	frame          int
	target         playdate.Bitmap
	screen, closed bool
}

func (*player) Info() (playdate.VideoInfo, error) {
	return playdate.VideoInfo{Width: 160, Height: 96, FrameRate: 10, FrameCount: 12}, nil
}
func (*player) LastError() (string, error)           { return "", nil }
func (p *player) SetContext(b playdate.Bitmap) error { p.target = b; p.screen = false; return nil }
func (p *player) UseScreenContext() error            { p.screen = true; return nil }
func (p *player) RenderFrame(f int) error            { p.frame = f; return nil }
func (p *player) Close() error                       { p.closed = true; return nil }

type audio struct {
	plays, stops   int
	paused, closed bool
}

func (a *audio) Play() error                     { a.plays++; return nil }
func (a *audio) Stop() error                     { a.stops++; return nil }
func (a *audio) Pause() error                    { a.paused = true; return nil }
func (a *audio) Resume() error                   { a.paused = false; return nil }
func (a *audio) Close() error                    { a.closed = true; return nil }
func (*audio) SetVolume(float32, float32) error  { return nil }
func (*audio) Volume() (float32, float32, error) { return 1, 1, nil }
func (a *audio) State() (playdate.PlaybackState, error) {
	if a.paused {
		return playdate.PlaybackPaused, nil
	}
	return playdate.PlaybackPlaying, nil
}
func (a *audio) PlayRepeated(int, float32) error { a.plays++; return nil }
func (*audio) Length() (float32, error)          { return 4, nil }
func (*audio) SetOffset(float32) error           { return nil }
func (*audio) Offset() (float32, error)          { return 0, nil }
func (*audio) SetRate(float32) error             { return nil }
func (*audio) Rate() (float32, error)            { return 1, nil }

type context struct {
	input  playdate.Input
	player *player
	audio  *audio
	bitmap *bitmap
	draws  int
}

func (*context) CurrentTimeMilliseconds() uint32                      { return 0 }
func (c *context) Input() playdate.Input                              { return c.input }
func (*context) Clear()                                               {}
func (*context) DrawText(string, int, int)                            {}
func (*context) LoadBitmap(string) (playdate.Bitmap, error)           { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error) { return nil, nil }
func (c *context) NewBitmap(int, int) (playdate.Bitmap, error) {
	c.bitmap = &bitmap{}
	return c.bitmap, nil
}
func (c *context) DrawBitmap(playdate.Bitmap, int, int) error                       { c.draws++; return nil }
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*context) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) { return nil, nil }
func (*context) UpdateAndDrawSprites()                                              {}
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error)               { return nil, nil }
func (c *context) LoadFilePlayer(string) (playdate.FilePlayer, error)               { return c.audio, nil }
func (c *context) LoadSamplePlayer(string) (playdate.SamplePlayer, error)           { return c.audio, nil }
func (c *context) LoadVideo(path string) (playdate.VideoPlayer, error)              { return c.player, nil }
func TestVideoPlaybackTargetsAndCleanup(t *testing.T) {
	p := &player{}
	a := &audio{}
	c := &context{player: p, audio: a}
	g := New().(*game)
	if err := g.Init(c); err != nil {
		t.Fatal(err)
	}
	if p.frame != 0 || p.target != c.bitmap {
		t.Fatal("initial frame was not rendered offscreen")
	}
	if a.plays != 1 {
		t.Fatal("audio did not start")
	}
	c.input = playdate.Input{Pressed: playdate.ButtonB}
	if _, err := g.Update(c); err != nil {
		t.Fatal(err)
	}
	if !p.screen {
		t.Fatal("B did not select screen context")
	}
	c.input = playdate.Input{Pressed: playdate.ButtonA | playdate.ButtonRight}
	if _, err := g.Update(c); err != nil {
		t.Fatal(err)
	}
	if !g.paused || p.frame != 1 {
		t.Fatalf("paused=%t frame=%d", g.paused, p.frame)
	}
	if a.plays != 1 || !a.paused {
		t.Fatalf("pause restarted audio: plays=%d paused=%t", a.plays, a.paused)
	}
	c.input = playdate.Input{Pressed: playdate.ButtonA}
	if _, err := g.Update(c); err != nil {
		t.Fatal(err)
	}
	if a.plays != 1 || a.paused {
		t.Fatalf("resume restarted audio: plays=%d paused=%t", a.plays, a.paused)
	}
	if err := g.HandleLifecycle(c, playdate.LifecycleTerminate); err != nil {
		t.Fatal(err)
	}
	if !p.closed || !a.closed || !c.bitmap.closed {
		t.Fatal("owned resources were not closed")
	}
}
