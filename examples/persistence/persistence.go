// Package persistence exercises the P4.2 versioned store contract.
package persistence

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	"github.com/ivan-gromov-dev/gopdsdk/playdate/store"
)

const (
	path          = "p4-store.bin"
	initial       = "score=1"
	migrated      = "score=1;sound=on"
	replacement   = "score=2;sound=off"
	resultSuccess = "P4.2 STORE OK"
)

var errPayload = errors.New("stored payload does not match")

type game struct{ result string }

// New creates the P4.2 persistence acceptance scene.
func New() playdate.Game { return &game{} }

func (game *game) Init(context playdate.Context) error {
	files, ok := any(context).(playdate.FileSystem)
	if !ok {
		game.result = "FAIL: capability unavailable"
		return nil
	}
	if err := exercise(files); err != nil {
		game.result = "FAIL: " + err.Error()
		return nil
	}
	game.result = resultSuccess
	return nil
}

func (game *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P4.2 PERSISTENCE", 112, 72)
	context.DrawText(game.result, 120, 112)
	context.DrawText("SAVE MIGRATE REPLACE LOAD", 76, 152)
	return true, nil
}

func exercise(files playdate.FileSystem) error {
	version1, err := store.New(files, store.Config{
		Path: path, Version: 1, MaximumSize: 64,
	})
	if err != nil {
		return err
	}
	if err = version1.Save([]byte(initial)); err != nil {
		return err
	}
	version2, err := store.New(files, store.Config{
		Path: path, Version: 2, MaximumSize: 64,
		Migrations: []store.VersionMigration{
			{From: 1, Migrate: func(payload []byte) ([]byte, error) {
				return append(payload, []byte(";sound=on")...), nil
			}},
		},
	})
	if err != nil {
		return err
	}
	payload, err := version2.Load()
	if err != nil {
		return err
	}
	if string(payload) != migrated {
		return errPayload
	}
	if err = version2.Save([]byte(replacement)); err != nil {
		return err
	}
	payload, err = version2.Load()
	if err != nil {
		return err
	}
	if string(payload) != replacement {
		return errPayload
	}
	return nil
}
