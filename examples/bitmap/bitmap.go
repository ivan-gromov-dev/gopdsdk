// Package bitmap exercises the portable P1.2 bitmap API on Simulator and device.
package bitmap

import (
	"errors"
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const assetPath = "images/acceptance"

type game struct {
	loaded  playdate.Bitmap
	created playdate.Bitmap
	width   int
	height  int
	closed  bool
}

// New creates the bitmap rendering acceptance game.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	loaded, err := context.LoadBitmap(assetPath)
	if err != nil {
		return err
	}
	g.loaded = loaded
	created, err := context.NewBitmap(48, 48)
	if err != nil {
		_ = loaded.Close()
		return err
	}
	g.created = created
	if err := created.Fill(playdate.ColorBlack); err != nil {
		return errors.Join(err, g.close())
	}
	g.width, err = loaded.Width()
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.height, err = loaded.Height()
	if err != nil {
		return errors.Join(err, g.close())
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P1.2 bitmap acceptance", 12, 8)
	context.DrawText("PDX: "+strconv.Itoa(g.width)+"x"+strconv.Itoa(g.height), 12, 30)
	context.DrawText("PASS load/create/fill/draw/scale", 12, 52)
	if err := context.DrawBitmap(g.loaded, 12, 82); err != nil {
		return false, err
	}
	if err := context.DrawScaledBitmap(g.loaded, 92, 82, 0.5, 0.5); err != nil {
		return false, err
	}
	if err := context.DrawBitmap(g.created, 150, 82); err != nil {
		return false, err
	}
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event != playdate.LifecycleTerminate {
		return nil
	}
	return g.close()
}

func (g *game) close() error {
	if g.closed {
		return nil
	}
	g.closed = true
	var err error
	if g.loaded != nil {
		err = g.loaded.Close()
	}
	if g.created != nil {
		err = errors.Join(err, g.created.Close())
	}
	return err
}
