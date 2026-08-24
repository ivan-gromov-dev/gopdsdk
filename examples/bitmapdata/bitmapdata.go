// Package bitmapdata exercises the complete P7.2 bitmap-data capability.
package bitmapdata

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	bitmapAsset = "images/acceptance"
	tableAsset  = "images/characters"
)

var errCapabilities = errors.New("P7.2 bitmap-data capability is unavailable")

type game struct {
	graphics playdate.BitmapDataGraphics
	bitmaps  []playdate.Bitmap
	table    playdate.BitmapTable
	frame    playdate.Bitmap
	collides bool
	dirty    bool
	masked   bool
	bytes    int
}

// New creates the complete P7.2 acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	graphics, ok := context.(playdate.BitmapDataGraphics)
	if !ok {
		return errCapabilities
	}
	g.graphics = graphics

	source, err := context.LoadBitmap(bitmapAsset)
	if err != nil {
		return err
	}
	g.bitmaps = append(g.bitmaps, source)
	copyBitmap, err := graphics.CopyBitmap(source)
	if err != nil {
		return g.fail(err)
	}
	g.bitmaps = append(g.bitmaps, copyBitmap)
	loadedInto, err := context.NewBitmap(64, 64)
	if err != nil {
		return g.fail(err)
	}
	g.bitmaps = append(g.bitmaps, loadedInto)
	if err = graphics.LoadIntoBitmap(bitmapAsset, loadedInto); err != nil {
		return g.fail(err)
	}
	mask, err := context.NewBitmap(64, 64)
	if err != nil {
		return g.fail(err)
	}
	g.bitmaps = append(g.bitmaps, mask)
	if err = mask.Fill(playdate.ColorWhite); err != nil {
		return g.fail(err)
	}
	if err = graphics.WithBitmapData(mask, func(data playdate.BitmapData) error {
		bytes, dataErr := data.Bytes()
		if dataErr != nil {
			return dataErr
		}
		for y := 0; y < data.Height(); y++ {
			for x := 0; x < data.Width(); x++ {
				if x < 8 || y < 8 || x >= 56 || y >= 56 {
					if dataErr = data.SetPixel(x, y, playdate.ColorWhite); dataErr != nil {
						return dataErr
					}
				}
			}
		}
		if len(bytes) > 0 {
			bytes[0] = 0
			if dataErr = data.MarkDirty(); dataErr != nil {
				return dataErr
			}
		}
		g.dirty, dataErr = data.Dirty()
		return dataErr
	}); err != nil {
		return g.fail(err)
	}
	if err = graphics.SetBitmapMask(copyBitmap, mask); err != nil {
		return g.fail(err)
	}
	g.masked = true
	maskView, present, err := graphics.BitmapMask(copyBitmap)
	if err != nil {
		return g.fail(err)
	}
	if !present {
		return g.fail(errors.New("P7.2 attached mask is missing"))
	}
	if err = graphics.WithBitmapData(maskView, func(data playdate.BitmapData) error {
		maskBytes, dataErr := data.Bytes()
		g.bytes = len(maskBytes)
		return dataErr
	}); err != nil {
		_ = maskView.Close()
		return g.fail(err)
	}
	if err = maskView.Close(); err != nil {
		return g.fail(err)
	}
	g.collides, err = graphics.CheckBitmapMaskCollision(source, 0, 0, playdate.BitmapUnflipped, copyBitmap, 0, 0, playdate.BitmapFlippedX, 0, 0, 64, 64)
	if err != nil {
		return g.fail(err)
	}
	rotated, allocation, err := graphics.RotatedBitmap(source, 30, 1, 1)
	if err != nil {
		return g.fail(err)
	}
	g.bytes += allocation
	g.bitmaps = append(g.bitmaps, rotated)
	snapshot, err := graphics.CopyDisplayBuffer()
	if err != nil {
		return g.fail(err)
	}
	g.bitmaps = append(g.bitmaps, snapshot)
	table, err := graphics.NewBitmapTable(4, 32, 32)
	if err != nil {
		return g.fail(err)
	}
	g.table = table
	if err = graphics.LoadIntoBitmapTable(tableAsset, table); err != nil {
		return g.fail(err)
	}
	g.frame, err = table.Frame(2)
	if err != nil {
		return g.fail(err)
	}
	return nil
}

func (g *game) fail(err error) error { return errors.Join(err, g.close()) }

func (g *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P7.2 bitmap data + masks", 12, 8)
	context.DrawText("PASS data dirty / copy / load-into", 12, 28)
	context.DrawText("PASS mask view / collision / rotate", 12, 48)
	context.DrawText("PASS table load-into / display copy", 12, 68)
	if !g.dirty || !g.collides || g.bytes == 0 {
		context.DrawText("FAIL runtime assertions", 12, 88)
		return false, errors.New("P7.2 runtime assertion failed")
	}
	positions := [][2]int{{16, 112}, {92, 112}, {168, 112}, {250, 104}, {340, 120}}
	for index, bitmap := range []playdate.Bitmap{g.bitmaps[0], g.bitmaps[1], g.bitmaps[2], g.bitmaps[4], g.frame} {
		if err := context.DrawBitmap(bitmap, positions[index][0], positions[index][1]); err != nil {
			return false, err
		}
	}
	if err := context.DrawScaledBitmap(g.bitmaps[5], 300, 154, .2, .2); err != nil {
		return false, err
	}
	context.DrawText("source  mask  loaded  rotated  table", 12, 202)
	return true, nil
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event != playdate.LifecycleTerminate {
		return nil
	}
	return g.close()
}

func (g *game) close() error {
	var result error
	if g.masked && len(g.bitmaps) > 1 && g.bitmaps[1] != nil && g.graphics != nil {
		result = g.graphics.ClearBitmapMask(g.bitmaps[1])
		g.masked = false
	}
	for index := len(g.bitmaps) - 1; index >= 0; index-- {
		if g.bitmaps[index] != nil {
			result = errors.Join(result, g.bitmaps[index].Close())
			g.bitmaps[index] = nil
		}
	}
	if g.table != nil {
		result = errors.Join(result, g.table.Close())
		g.table, g.frame = nil, nil
	}
	return result
}
