// Package systemmenu exercises the P4.3 menu, localization, and persistence contract.
package systemmenu

import (
	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	"github.com/ivan-gromov-dev/gopdsdk/playdate/store"
)

type game struct {
	result   string
	settings *store.Store
	sound    playdate.CheckmarkMenuItem
	mode     playdate.OptionsMenuItem
}

// New creates the P4.3 acceptance scene.
func New() playdate.Game { return &game{} }

func (game *game) Init(context playdate.Context) error {
	menu, menuOK := any(context).(playdate.SystemMenu)
	localization, localizationOK := any(context).(playdate.Localization)
	files, filesOK := any(context).(playdate.FileSystem)
	if !menuOK || !localizationOK || !filesOK {
		game.result = "FAIL: capability unavailable"
		return nil
	}
	settings, err := store.New(files, store.Config{Path: "p4-menu.bin", Version: 1, MaximumSize: 2})
	if err != nil {
		return err
	}
	game.settings = settings
	values := []byte{1, 0}
	if saved, loadErr := settings.Load(); loadErr == nil && len(saved) == 2 {
		copy(values, saved)
	}
	text := func(key, fallback string) string {
		if value, ok := localization.LocalizedText(key, playdate.LanguageSystem); ok {
			return value
		}
		return fallback
	}
	game.sound, err = menu.AddCheckmarkMenuItem(text("sound", "Sound"), values[0] != 0, func() { _ = game.save() })
	if err != nil {
		return err
	}
	game.mode, err = menu.AddOptionsMenuItem(text("mode", "Mode"), []string{text("easy", "Easy"), text("hard", "Hard")}, func() { _ = game.save() })
	if err != nil {
		return err
	}
	if err = game.mode.SetValue(int(values[1])); err != nil {
		return err
	}
	if err = game.save(); err != nil {
		return err
	}
	game.result = "P4.3 MENU OK"
	return nil
}

func (game *game) save() error {
	sound := byte(0)
	if game.sound.Value() {
		sound = 1
	}
	return game.settings.Save([]byte{sound, byte(game.mode.Value())})
}

func (game *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P4.3 SYSTEM MENU", 112, 72)
	context.DrawText(game.result, 120, 112)
	context.DrawText("OPEN MENU TO CHANGE", 92, 152)
	return true, nil
}
