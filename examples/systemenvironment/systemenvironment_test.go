package systemenvironment

import (
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type context struct {
	input  playdate.Input
	lines  []string
	resets int
}

func (*context) CurrentTimeMilliseconds() uint32                                    { return 0 }
func (*context) Clear()                                                             {}
func (context *context) DrawText(text string, _, _ int)                             { context.lines = append(context.lines, text) }
func (*context) LoadBitmap(string) (playdate.Bitmap, error)                         { return nil, nil }
func (*context) LoadBitmapTable(string) (playdate.BitmapTable, error)               { return nil, nil }
func (*context) NewBitmap(int, int) (playdate.Bitmap, error)                        { return nil, nil }
func (*context) DrawBitmap(playdate.Bitmap, int, int) error                         { return nil }
func (*context) DrawScaledBitmap(playdate.Bitmap, int, int, float32, float32) error { return nil }
func (*context) NewSprite() (playdate.Sprite, error)                                { return nil, nil }
func (*context) QuerySpritesAtPoint(float32, float32) []playdate.Sprite             { return nil }
func (*context) QuerySpritesInRect(playdate.Rect) []playdate.Sprite                 { return nil }
func (*context) QueryOverlappingSprites(playdate.Sprite) ([]playdate.Sprite, error) { return nil, nil }
func (*context) UpdateAndDrawSprites()                                              {}
func (*context) LoadSoundEffect(string) (playdate.SoundEffect, error)               { return nil, nil }
func (*context) LoadFilePlayer(string) (playdate.FilePlayer, error)                 { return nil, nil }
func (context *context) Input() playdate.Input                                      { return context.input }
func (*context) CurrentEpochTime() playdate.EpochTime {
	return playdate.EpochTime{Seconds: 762566400, Milliseconds: 7}
}
func (*context) EpochToDateTime(uint32) playdate.DateTime {
	return playdate.DateTime{Year: 2024, Month: 3, Day: 1, Weekday: 5, Hour: 12, Minute: 34, Second: 56}
}
func (*context) DateTimeToEpoch(playdate.DateTime) (uint32, error) { return 762566400, nil }
func (context *context) ResetElapsedTime()                         { context.resets++ }
func (*context) ElapsedTime() float32                              { return 1.25 }
func (*context) SystemInfo() playdate.SystemInfo {
	return playdate.SystemInfo{OSVersion: 30101, Language: playdate.LanguageEnglish, PDXVersion: 30100}
}

func TestAcceptanceDisplaysOwnedEnvironmentAndResetsTimer(t *testing.T) {
	probe := New().(*game)
	context := &context{}
	if err := probe.Init(context); err != nil {
		t.Fatal(err)
	}
	context.input.Pressed = playdate.ButtonA
	if _, err := probe.Update(context); err != nil {
		t.Fatal(err)
	}
	if context.resets != 2 {
		t.Fatalf("timer resets = %d, want 2", context.resets)
	}
	want := []string{"EPOCH 762566400.007", "2024-03-01 12:34:56 W5", "ROUNDTRIP 762566400", "ELAPSED 1.250", "OS 30101 PDX 30100"}
	for index, value := range want {
		if context.lines[index+1] != value {
			t.Errorf("line %d = %q, want %q", index+1, context.lines[index+1], value)
		}
	}
}
