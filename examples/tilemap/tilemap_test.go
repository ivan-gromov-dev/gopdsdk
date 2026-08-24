package tilemap

import (
	"math"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func testWorld(t *testing.T) *playdate.TileMap {
	t.Helper()
	const columns, rows = 32, 10
	tiles := make([]uint8, columns*rows)
	for column := 0; column < columns; column++ {
		tiles[(rows-1)*columns+column] = 2
	}
	tiles[(rows-2)*columns+3] = 2
	world, err := playdate.NewTileMap(playdate.TileMapConfig{
		Columns: columns, Rows: rows, TileWidth: tileSize, TileHeight: tileSize,
		Tiles: tiles, Solid: []bool{false, false, true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return world
}

func TestJumpClearsTileAndCameraFollows(t *testing.T) {
	g := &game{
		world: testWorld(t), camera: playdate.Camera{Width: 400, Height: 240},
		playerX: 40, playerY: groundPlayerY, grounded: true,
	}
	minimumY := g.playerY
	for frame := 0; frame < 70; frame++ {
		input := playdate.Input{Buttons: playdate.ButtonRight, DeltaSeconds: 1.0 / 30.0}
		if frame == 5 {
			input.Pressed = playdate.ButtonA
		}
		g.advance(input)
		if g.playerY < minimumY {
			minimumY = g.playerY
		}
	}
	if minimumY >= groundPlayerY-32 {
		t.Fatalf("jump height = %f", groundPlayerY-minimumY)
	}
	if g.playerX <= 128 {
		t.Fatalf("player did not clear obstacle: x=%f", g.playerX)
	}
	if g.camera.X <= 0 {
		t.Fatalf("camera did not follow player: %+v", g.camera)
	}
	if g.playerY > groundPlayerY || !g.grounded {
		t.Fatalf("player did not land: y=%f grounded=%t", g.playerY, g.grounded)
	}
}

func TestCannotWalkThroughSolidTile(t *testing.T) {
	g := &game{
		world: testWorld(t), camera: playdate.Camera{Width: 400, Height: 240},
		playerX: 70, playerY: groundPlayerY, grounded: true,
	}
	for range 20 {
		g.advance(playdate.Input{Buttons: playdate.ButtonRight, DeltaSeconds: 1.0 / 30.0})
	}
	if g.playerX+playerSize > 96 {
		t.Fatalf("player crossed obstacle without jumping: x=%f", g.playerX)
	}
}

func TestNonFiniteFrameDeltaUsesFallback(t *testing.T) {
	g := &game{
		world: testWorld(t), camera: playdate.Camera{Width: 400, Height: 240},
		playerX: 40, playerY: groundPlayerY, grounded: true,
	}
	g.advance(playdate.Input{Buttons: playdate.ButtonRight, DeltaSeconds: float32(math.NaN())})
	if math.IsNaN(float64(g.playerX)) || math.IsNaN(float64(g.playerY)) {
		t.Fatalf("non-finite player state: x=%f y=%f", g.playerX, g.playerY)
	}
	if g.playerX <= 40 {
		t.Fatalf("fallback frame did not move player: x=%f", g.playerX)
	}
}
