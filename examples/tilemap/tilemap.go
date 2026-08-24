// Package tilemap exercises the P3.3 camera, bounded tile rendering, and static collision API.
package tilemap

import (
	"errors"
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	tileSize      = 32
	playerSize    = 18
	moveSpeed     = 120
	jumpSpeed     = 380
	gravity       = 1100
	groundPlayerY = 270
)

var errCapabilities = errors.New("P3.3 graphics capabilities are unavailable")

type game struct {
	world     *playdate.TileMap
	camera    playdate.Camera
	tiles     []playdate.Bitmap
	playerX   float32
	playerY   float32
	velocityY float32
	grounded  bool
	black     playdate.Paint
	white     playdate.Paint
}

// New creates the P3.3 tile-map and camera acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	offscreen, offscreenOK := context.(playdate.OffscreenGraphics)
	primitives, primitivesOK := context.(playdate.PrimitiveGraphics)
	if !offscreenOK || !primitivesOK {
		return errCapabilities
	}
	black, err := playdate.SolidPaint(playdate.ColorBlack)
	if err != nil {
		return err
	}
	white, err := playdate.SolidPaint(playdate.ColorWhite)
	if err != nil {
		return err
	}
	g.black, g.white = black, white
	for index := 0; index < 2; index++ {
		bitmap, createErr := context.NewBitmap(tileSize, tileSize)
		if createErr != nil {
			return errors.Join(createErr, g.close())
		}
		g.tiles = append(g.tiles, bitmap)
		if fillErr := bitmap.Fill(playdate.ColorWhite); fillErr != nil {
			return errors.Join(fillErr, g.close())
		}
		if drawErr := offscreen.DrawInto(bitmap, func() error {
			if index == 0 {
				return primitives.DrawRect(0, 0, tileSize, tileSize, black)
			}
			return primitives.FillRect(0, 0, tileSize, tileSize, black)
		}); drawErr != nil {
			return errors.Join(drawErr, g.close())
		}
	}

	const columns, rows = 32, 10
	tiles := make([]uint8, columns*rows)
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			if row == rows-1 || column%7 == 3 && row == rows-2 {
				tiles[row*columns+column] = 2
			} else if (row+column)%3 == 0 {
				tiles[row*columns+column] = 1
			}
		}
	}
	g.world, err = playdate.NewTileMap(playdate.TileMapConfig{Columns: columns, Rows: rows, TileWidth: tileSize, TileHeight: tileSize, Tiles: tiles, Solid: []bool{false, false, true}})
	g.camera = playdate.Camera{Width: 400, Height: 240}
	g.playerX, g.playerY, g.grounded = 40, groundPlayerY, true
	return err
}

func (g *game) Update(context playdate.Context) (bool, error) {
	g.advance(context.Input())
	context.Clear()
	if _, err := g.world.Draw(context, g.tiles, g.camera); err != nil {
		return false, err
	}
	primitives := context.(playdate.PrimitiveGraphics)
	playerScreenX, playerScreenY := int(g.playerX)-g.camera.X, int(g.playerY)-g.camera.Y
	if err := primitives.FillRect(playerScreenX, playerScreenY, playerSize, playerSize, g.black); err != nil {
		return false, err
	}
	if err := primitives.FillRect(playerScreenX+4, playerScreenY+4, playerSize-8, playerSize-8, g.white); err != nil {
		return false, err
	}
	context.DrawText("Left/Right move, A jumps", 10, 8)
	context.DrawText("x="+strconv.Itoa(int(g.playerX))+" y="+strconv.Itoa(int(g.playerY))+" buttons="+strconv.Itoa(int(context.Input().Buttons)), 10, 28)
	return true, nil
}

func (g *game) advance(input playdate.Input) {
	deltaSeconds := input.DeltaSeconds
	if deltaSeconds <= 0 || deltaSeconds > 0.05 || deltaSeconds != deltaSeconds {
		deltaSeconds = 1.0 / 30.0
	}
	deltaX := float32(0)
	if input.Buttons.Has(playdate.ButtonLeft) {
		deltaX = -moveSpeed * deltaSeconds
	} else if input.Buttons.Has(playdate.ButtonRight) {
		deltaX = moveSpeed * deltaSeconds
	}
	if input.Pressed.Has(playdate.ButtonA) && g.grounded {
		g.velocityY, g.grounded = -jumpSpeed, false
	}

	horizontal := playdate.Rect{X: g.playerX + deltaX, Y: g.playerY, Width: playerSize, Height: playerSize}
	if !g.world.IntersectsSolid(horizontal) {
		g.playerX += deltaX
	}
	g.velocityY += gravity * deltaSeconds
	deltaY := g.velocityY * deltaSeconds
	vertical := playdate.Rect{X: g.playerX, Y: g.playerY + deltaY, Width: playerSize, Height: playerSize}
	if g.world.IntersectsSolid(vertical) {
		g.grounded = g.velocityY > 0
		g.velocityY = 0
	} else {
		g.playerY += deltaY
		g.grounded = false
	}

	g.camera.X = int(g.playerX) - 200
	g.camera.Y = int(g.playerY) - 160
	width, height := g.world.WorldSize()
	g.camera.Clamp(width, height)
}

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecycleTerminate {
		return g.close()
	}
	return nil
}

func (g *game) close() error {
	var result error
	for _, bitmap := range g.tiles {
		result = errors.Join(result, bitmap.Close())
	}
	g.tiles = nil
	return result
}
