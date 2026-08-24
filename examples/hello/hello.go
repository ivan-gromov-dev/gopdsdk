// Package hello provides the minimal gopdsdk Simulator example.
package hello

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

type game struct{}

// New creates the Hello World game expected by gopdsdk build.
func New() playdate.Game { return game{} }

func (game) Init(playdate.Context) error { return nil }

func (game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("Hello from gopdsdk", 16, 16)
	return true, nil
}
