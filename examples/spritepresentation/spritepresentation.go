// Package spritepresentation visually exercises P8 sprite presentation,
// queries, and display-list controls.
package spritepresentation

import (
	"errors"
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

var errCapabilities = errors.New("P8 sprite capabilities are unavailable")

type game struct {
	bitmaps           []playdate.Bitmap
	tables            []playdate.BitmapTable
	tileMaps          []playdate.SpriteTileMap
	sprites           []playdate.Sprite
	display           playdate.Display
	queries           playdate.SpriteQueries
	displayList       playdate.SpriteDisplayList
	updatesEnabled    bool
	collisionsEnabled bool
	offset            bool
	stencils          bool
	clip              bool
	closed            bool
	frame             int
	tileValue         uint16
	callbackUpdates   int
}

// New creates the P8 sprite-presentation acceptance scene.
func New() playdate.Game {
	return &game{updatesEnabled: true, collisionsEnabled: true, stencils: true, clip: true}
}

func (g *game) Init(context playdate.Context) error {
	offscreen, offscreenOK := context.(playdate.OffscreenGraphics)
	primitives, primitivesOK := context.(playdate.PrimitiveGraphics)
	nativeTileMaps, tileMapsOK := context.(playdate.SpriteTileMaps)
	redraw, redrawOK := context.(playdate.SpriteRedraw)
	display, displayOK := context.(playdate.Display)
	queries, queriesOK := context.(playdate.SpriteQueries)
	displayList, displayListOK := context.(playdate.SpriteDisplayList)
	if !offscreenOK || !primitivesOK || !tileMapsOK || !redrawOK || !displayOK || !queriesOK || !displayListOK {
		return errCapabilities
	}
	redraw.SetAlwaysRedraw(true)
	g.display = display
	g.queries = queries
	g.displayList = displayList
	base, err := context.NewBitmap(48, 32)
	if err != nil {
		return err
	}
	g.bitmaps = append(g.bitmaps, base)
	stencil, err := context.NewBitmap(400, 240)
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.bitmaps = append(g.bitmaps, stencil)
	black, err := playdate.SolidPaint(playdate.ColorBlack)
	if err != nil {
		return errors.Join(err, g.close())
	}
	white, err := playdate.SolidPaint(playdate.ColorWhite)
	if err != nil {
		return errors.Join(err, g.close())
	}
	if err := base.Fill(playdate.ColorWhite); err != nil {
		return errors.Join(err, g.close())
	}
	if err := offscreen.DrawInto(base, func() error {
		if err := primitives.FillRect(4, 4, 10, 24, black); err != nil {
			return err
		}
		if err := primitives.FillRect(4, 18, 34, 10, black); err != nil {
			return err
		}
		return primitives.FillTriangle(34, 12, 46, 23, 34, 31, black)
	}); err != nil {
		return errors.Join(err, g.close())
	}
	if err := stencil.Fill(playdate.ColorBlack); err != nil {
		return errors.Join(err, g.close())
	}
	if err := offscreen.DrawInto(stencil, func() error {
		return primitives.FillEllipse(134, 126, 32, 32, 0, 360, white)
	}); err != nil {
		return errors.Join(err, g.close())
	}

	setups := []func(playdate.Sprite) error{
		func(s playdate.Sprite) error { return setup(s, base, 55, 72, playdate.BitmapUnflipped) },
		func(s playdate.Sprite) error { return setup(s, base, 150, 72, playdate.BitmapFlippedX) },
		func(s playdate.Sprite) error { return setup(s, base, 245, 72, playdate.BitmapFlippedY) },
		func(s playdate.Sprite) error { return setup(s, base, 340, 72, playdate.BitmapFlippedXY) },
		func(s playdate.Sprite) error {
			if err := setup(s, base, 55, 142, playdate.BitmapUnflipped); err != nil {
				return err
			}
			return s.SetStencilPattern([8]byte{0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55})
		},
		func(s playdate.Sprite) error {
			if err := setup(s, base, 150, 142, playdate.BitmapUnflipped); err != nil {
				return err
			}
			return s.SetStencilImage(stencil, false)
		},
		func(s playdate.Sprite) error {
			if err := setup(s, base, 245, 142, playdate.BitmapUnflipped); err != nil {
				return err
			}
			return s.SetClipRect(221, 126, 24, 32)
		},
		func(s playdate.Sprite) error {
			if err := setup(s, base, 340, 142, playdate.BitmapUnflipped); err != nil {
				return err
			}
			if err := s.SetDrawMode(playdate.DrawModeInverted); err != nil {
				return err
			}
			return s.SetOpaque(true)
		},
		func(s playdate.Sprite) error {
			if err := setup(s, base, 105, 188, playdate.BitmapUnflipped); err != nil {
				return err
			}
			return s.SetIgnoresDrawOffset(false)
		},
		func(s playdate.Sprite) error {
			if err := setup(s, base, 295, 188, playdate.BitmapUnflipped); err != nil {
				return err
			}
			return s.SetIgnoresDrawOffset(true)
		},
	}
	for _, configure := range setups {
		sprite, createErr := context.NewSprite()
		if createErr != nil {
			return errors.Join(createErr, g.close())
		}
		g.sprites = append(g.sprites, sprite)
		if configureErr := configure(sprite); configureErr != nil {
			return errors.Join(configureErr, g.close())
		}
	}
	procedural, err := context.NewSprite()
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.sprites = append(g.sprites, procedural)
	if err := procedural.SetBounds(playdate.Rect{X: 188, Y: 36, Width: 24, Height: 24}); err != nil {
		return errors.Join(err, g.close())
	}
	if err := procedural.SetPosition(200, 48); err != nil {
		return errors.Join(err, g.close())
	}
	if err := procedural.SetCollideRect(playdate.Rect{Width: 24, Height: 24}); err != nil {
		return errors.Join(err, g.close())
	}
	if err := procedural.SetDrawCallback(func(_ playdate.Sprite, bounds, _ playdate.Rect) {
		_ = primitives.FillEllipse(int(bounds.X), int(bounds.Y), int(bounds.Width), int(bounds.Height), 0, 360, black)
	}); err != nil {
		return errors.Join(err, g.close())
	}
	if err := procedural.SetUpdateCallback(func(playdate.Sprite) { g.callbackUpdates++ }); err != nil {
		return errors.Join(err, g.close())
	}
	if err := procedural.SetCollisionResponseCallback(func(_, _ playdate.Sprite) playdate.CollisionResponse { return playdate.CollisionBounce }); err != nil {
		return errors.Join(err, g.close())
	}
	if err := procedural.Add(); err != nil {
		return errors.Join(err, g.close())
	}
	table, err := context.LoadBitmapTable("images/characters")
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.tables = append(g.tables, table)
	tileMap, err := nativeTileMaps.NewSpriteTileMap(table, 2, 1, []uint16{0, 1})
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.tileMaps = append(g.tileMaps, tileMap)
	tileSprite, err := context.NewSprite()
	if err != nil {
		return errors.Join(err, g.close())
	}
	g.sprites = append(g.sprites, tileSprite)
	if err := tileSprite.SetCenter(.5, .5); err != nil {
		return errors.Join(err, g.close())
	}
	if err := tileSprite.SetBounds(playdate.Rect{X: 168, Y: 172, Width: 64, Height: 32}); err != nil {
		return errors.Join(err, g.close())
	}
	if err := tileSprite.SetPosition(200, 188); err != nil {
		return errors.Join(err, g.close())
	}
	if err := tileSprite.SetTileMap(tileMap); err != nil {
		return errors.Join(err, g.close())
	}
	if err := tileSprite.Add(); err != nil {
		return errors.Join(err, g.close())
	}
	got, ok, err := tileSprite.TileMap()
	if err != nil || !ok || got != tileMap {
		return errors.Join(errors.New("tilemap getter mismatch"), err)
	}
	if err := g.verifyGetters(); err != nil {
		return err
	}
	return g.verifyP82()
}

func setup(sprite playdate.Sprite, bitmap playdate.Bitmap, x, y float32, flip playdate.BitmapFlip) error {
	if err := sprite.SetBitmap(bitmap); err != nil {
		return err
	}
	if err := sprite.SetCenter(.5, .5); err != nil {
		return err
	}
	if err := sprite.SetBounds(playdate.Rect{X: x - 24, Y: y - 16, Width: 48, Height: 32}); err != nil {
		return err
	}
	if err := sprite.SetPosition(x, y); err != nil {
		return err
	}
	if err := sprite.SetVisible(true); err != nil {
		return err
	}
	if err := sprite.SetZIndex(1); err != nil {
		return err
	}
	if err := sprite.SetImageFlip(flip); err != nil {
		return err
	}
	if err := sprite.SetDrawMode(playdate.DrawModeCopy); err != nil {
		return err
	}
	if err := sprite.SetOpaque(false); err != nil {
		return err
	}
	if err := sprite.SetUpdatesEnabled(true); err != nil {
		return err
	}
	if err := sprite.SetCollisionsEnabled(true); err != nil {
		return err
	}
	if err := sprite.SetCollideRect(playdate.Rect{Width: 48, Height: 32}); err != nil {
		return err
	}
	if err := sprite.SetTag(81); err != nil {
		return err
	}
	return sprite.Add()
}

func (g *game) verifyGetters() error {
	s := g.sprites[0]
	x, y, err := s.Center()
	if err != nil || x != .5 || y != .5 {
		return errors.Join(errors.New("center getter mismatch"), err)
	}
	bounds, err := s.Bounds()
	if err != nil || bounds.Width != 48 || bounds.Height != 32 {
		return errors.Join(errors.New("bounds getter mismatch"), err)
	}
	x, y, err = s.Position()
	if err != nil || x != 55 || y != 72 {
		return errors.Join(errors.New("position getter mismatch"), err)
	}
	visible, err := s.Visible()
	if err != nil || !visible {
		return errors.Join(errors.New("visibility getter mismatch"), err)
	}
	z, err := s.ZIndex()
	if err != nil || z != 1 {
		return errors.Join(errors.New("z-index getter mismatch"), err)
	}
	flip, err := s.ImageFlip()
	if err != nil || flip != playdate.BitmapUnflipped {
		return errors.Join(errors.New("flip getter mismatch"), err)
	}
	if _, err = s.CollideRect(); err != nil {
		return err
	}
	tag, err := s.Tag()
	if err != nil || tag != 81 {
		return errors.Join(errors.New("tag getter mismatch"), err)
	}
	return nil
}

func (g *game) verifyP82() error {
	if count := g.displayList.SpriteCount(); count != len(g.sprites) {
		return errors.New("P8.2 sprite count mismatch: " + strconv.Itoa(count))
	}
	line := g.queries.QuerySpritesAlongLine(0, 72, 399, 72)
	detailed := g.queries.QuerySpriteInfoAlongLine(0, 72, 399, 72)
	if len(line) != 4 || len(detailed) != len(line) {
		return errors.New("P8.2 line query mismatch")
	}
	for index, hit := range detailed {
		if hit.Sprite == nil || hit.EntryTime > hit.ExitTime || index > 0 && detailed[index-1].EntryTime > hit.EntryTime {
			return errors.New("P8.2 detailed line hit mismatch")
		}
	}
	x, y, err := g.sprites[0].Position()
	if err != nil {
		return err
	}
	checked, err := g.sprites[0].CheckCollisions(150, 72)
	if err != nil || len(checked.Collisions) == 0 {
		return errors.Join(errors.New("P8.2 collision check mismatch"), err)
	}
	actualX, actualY, err := g.sprites[0].Position()
	if err != nil || actualX != x || actualY != y {
		return errors.Join(errors.New("P8.2 collision check moved sprite"), err)
	}
	batch := g.sprites[:2]
	if err := g.displayList.RemoveSprites(batch); err != nil {
		return err
	}
	if count := g.displayList.SpriteCount(); count != len(g.sprites)-len(batch) {
		return errors.New("P8.2 bulk remove mismatch")
	}
	if err := g.displayList.AddSprites(batch); err != nil {
		return err
	}
	g.displayList.RemoveAllSprites()
	if count := g.displayList.SpriteCount(); count != 0 {
		return errors.New("P8.2 remove-all mismatch")
	}
	if err := g.displayList.AddSprites(g.sprites); err != nil {
		return err
	}
	g.displayList.ResetCollisionWorld()
	for _, sprite := range g.sprites[:len(g.sprites)-1] {
		if err := sprite.SetCollideRect(playdate.Rect{Width: 48, Height: 32}); err != nil {
			return err
		}
	}
	callbackSprite := g.sprites[len(g.sprites)-2]
	result, err := callbackSprite.CheckCollisions(55, 72)
	if err != nil || len(result.Collisions) == 0 || result.Collisions[0].ResponseType != playdate.CollisionBounce {
		return errors.Join(errors.New("P8.3 collision callback mismatch"), err)
	}
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	g.frame++
	input := context.Input()
	if input.Pressed.Has(playdate.ButtonA) {
		g.updatesEnabled = !g.updatesEnabled
		for _, sprite := range g.sprites {
			if err := sprite.SetUpdatesEnabled(g.updatesEnabled); err != nil {
				return false, err
			}
			actual, err := sprite.UpdatesEnabled()
			if err != nil || actual != g.updatesEnabled {
				return false, errors.Join(errors.New("updates-enabled getter mismatch"), err)
			}
		}
	}
	if input.Pressed.Has(playdate.ButtonB) {
		g.collisionsEnabled = !g.collisionsEnabled
		for _, sprite := range g.sprites {
			if err := sprite.SetCollisionsEnabled(g.collisionsEnabled); err != nil {
				return false, err
			}
			actual, err := sprite.CollisionsEnabled()
			if err != nil || actual != g.collisionsEnabled {
				return false, errors.Join(errors.New("collisions-enabled getter mismatch"), err)
			}
		}
	}
	if input.Pressed.Has(playdate.ButtonUp) {
		g.stencils = !g.stencils
		if g.stencils {
			if err := g.sprites[4].SetStencilPattern([8]byte{0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55}); err != nil {
				return false, err
			}
			if err := g.sprites[5].SetStencilImage(g.bitmaps[1], false); err != nil {
				return false, err
			}
		} else {
			if err := g.sprites[4].ClearStencil(); err != nil {
				return false, err
			}
			if err := g.sprites[5].ClearStencil(); err != nil {
				return false, err
			}
		}
	}
	if input.Pressed.Has(playdate.ButtonDown) {
		g.clip = !g.clip
		if g.clip {
			if err := g.sprites[6].SetClipRect(221, 126, 24, 32); err != nil {
				return false, err
			}
		} else if err := g.sprites[6].ClearClipRect(); err != nil {
			return false, err
		}
	}
	if input.Pressed.Has(playdate.ButtonLeft) || input.Pressed.Has(playdate.ButtonRight) {
		g.offset = !g.offset
		if g.offset {
			g.display.SetOffset(24, 0)
		} else {
			g.display.SetOffset(0, 0)
		}
	}
	if g.frame%30 == 0 {
		value := uint16(0)
		if (g.frame/30)%2 == 0 {
			value = 1
		}
		if err := g.tileMaps[0].SetTile(0, 0, value); err != nil {
			return false, err
		}
		actual, err := g.tileMaps[0].Tile(0, 0)
		if err != nil || actual != value {
			return false, errors.Join(errors.New("native tile mutation mismatch"), err)
		}
		g.tileValue = actual
		tileSprite := g.sprites[len(g.sprites)-1]
		if err := tileSprite.Remove(); err != nil {
			return false, err
		}
		if err := tileSprite.ClearTileMap(); err != nil {
			return false, err
		}
		if err := tileSprite.SetTileMap(g.tileMaps[0]); err != nil {
			return false, err
		}
		if err := tileSprite.Add(); err != nil {
			return false, err
		}
		if err := tileSprite.MarkDirty(); err != nil {
			return false, err
		}
	}
	context.Clear()
	context.UpdateAndDrawSprites()
	context.DrawText("P8 SPRITES  P8.3 PASS "+strconv.Itoa(g.callbackUpdates), 6, 4)
	context.DrawText("FLIP: NONE        X          Y          XY", 6, 26)
	context.DrawText("PATTERN       IMAGE MASK      CLIP HALF      INVERT+OPAQUE", 6, 98)
	context.DrawText("OFFSET MOVES     TILEMAP "+strconv.Itoa(int(g.tileValue))+"       OFFSET IGNORED", 6, 164)
	context.DrawText("A UPDATE "+onOff(g.updatesEnabled)+"   B COLLIDE "+onOff(g.collisionsEnabled), 6, 204)
	context.DrawText("UP STENCIL "+onOff(g.stencils)+"  DOWN CLIP "+onOff(g.clip)+"  LEFT/RIGHT OFFSET "+onOff(g.offset), 6, 220)
	return true, nil
}

func onOff(value bool) string { return map[bool]string{true: "ON", false: "OFF"}[value] }

func (g *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecycleTerminate {
		return g.close()
	}
	return nil
}

func (g *game) close() error {
	if g.closed {
		return nil
	}
	g.closed = true
	g.display.SetOffset(0, 0)
	var result error
	for _, sprite := range g.sprites {
		result = errors.Join(result, sprite.Close())
	}
	for _, tileMap := range g.tileMaps {
		result = errors.Join(result, tileMap.Close())
	}
	for _, table := range g.tables {
		result = errors.Join(result, table.Close())
	}
	for _, bitmap := range g.bitmaps {
		result = errors.Join(result, bitmap.Close())
	}
	return result
}
