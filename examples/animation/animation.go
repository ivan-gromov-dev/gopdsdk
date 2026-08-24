// Package animation exercises table-backed delta-time animation.
package animation

import (
	"errors"
	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const tableAsset = "images/characters"

type game struct {
	table     playdate.BitmapTable
	player    *playdate.Animation
	obstacles [2]*playdate.Animation
}

// New creates the P2.3 animation acceptance game.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	table, err := context.LoadBitmapTable(tableAsset)
	if err != nil {
		return err
	}
	g.table = table
	if g.player, err = playdate.NewAnimation(table, 0, 2, 0.12); err != nil {
		return errors.Join(err, table.Close())
	}
	for index := range g.obstacles {
		g.obstacles[index], err = playdate.NewAnimation(table, 0, 4, 1)
		if err == nil {
			err = g.obstacles[index].SetFixedFrame(2 + index)
		}
		if err != nil {
			return errors.Join(err, table.Close())
		}
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	g.player.Update(context.Input().DeltaSeconds)
	player, err := g.player.Bitmap()
	if err != nil {
		return false, err
	}
	left, err := g.obstacles[0].Bitmap()
	if err != nil {
		return false, err
	}
	right, err := g.obstacles[1].Bitmap()
	if err != nil {
		return false, err
	}
	context.Clear()
	context.DrawText("P2.3 table animation", 12, 8)
	if err = context.DrawBitmap(player, 184, 100); err != nil {
		return false, err
	}
	if err = context.DrawBitmap(left, 80, 160); err != nil {
		return false, err
	}
	if err = context.DrawBitmap(right, 288, 160); err != nil {
		return false, err
	}
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecyclePause {
		g.player.Pause()
	}
	if event == playdate.LifecycleResume {
		g.player.Resume()
	}
	if event == playdate.LifecycleTerminate && g.table != nil {
		return g.table.Close()
	}
	return nil
}
