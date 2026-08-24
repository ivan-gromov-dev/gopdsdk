// Package collision is the deterministic P2.2 collision acceptance game.
package collision

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	playerSize       = float32(16)
	wallGroup        = uint32(1 << 0)
	collectibleGroup = uint32(1 << 1)
	WallType         = uint32(1 << 0)
	CollectibleType  = uint32(1 << 1)
)

// Object is one queryable collision item in the pure-Go plan.
type Object struct {
	Rect        playdate.Rect
	Type, Group uint32
}

// Filter selects object types and groups; zero masks match every value.
type Filter struct{ Types, Groups uint32 }

// QueryPoint returns stable scene indices matching a point and filter.
func QueryPoint(objects []Object, point playdate.Point, filter Filter) []int {
	return query(objects, playdate.Rect{X: point.X, Y: point.Y, Width: .0001, Height: .0001}, filter)
}

// QueryRect returns stable scene indices overlapping a rectangle.
func QueryRect(objects []Object, rect playdate.Rect, filter Filter) []int {
	return query(objects, rect, filter)
}

// QuerySprite returns stable scene indices overlapping a sprite rectangle.
func QuerySprite(objects []Object, sprite playdate.Rect, filter Filter) []int {
	return query(objects, sprite, filter)
}

func query(objects []Object, rect playdate.Rect, filter Filter) []int {
	var result []int
	for index, object := range objects {
		if filter.Types != 0 && object.Type&filter.Types == 0 {
			continue
		}
		if filter.Groups != 0 && object.Group&filter.Groups == 0 {
			continue
		}
		if overlaps(rect, object.Rect) {
			result = append(result, index)
		}
	}
	return result
}

// State is the pure-Go gameplay state used by tests and the native game.
type State struct {
	Player    playdate.Rect
	Collected uint8
	Score     int
}

// Frame is one deterministic movement request.
type Frame struct {
	DX, DY float32
	Reset  bool
}

// DrawCommand is one deterministic text-mode acceptance draw.
type DrawCommand struct {
	Text string
	X, Y int
}

// DrawPlan derives the visible player, walls, collectibles, score, and reset hint.
func DrawPlan(state State) []DrawCommand {
	result := []DrawCommand{{Text: "P2.2 collision  score:" + strconv.Itoa(state.Score) + "  B:reset", X: 12, Y: 8}}
	for _, wall := range walls {
		result = append(result, wallDrawPlan(wall)...)
	}
	for index, item := range collectibles {
		if state.Collected&(1<<index) == 0 {
			result = append(result, DrawCommand{Text: "*", X: int(item.X), Y: int(item.Y)})
		}
	}
	return append(result, DrawCommand{Text: "@", X: int(state.Player.X), Y: int(state.Player.Y)})
}

func wallDrawPlan(wall playdate.Rect) []DrawCommand {
	const tile = float32(8)
	result := make([]DrawCommand, 0, int(wall.Width/tile)*int(wall.Height/tile))
	for y := wall.Y; y < wall.Y+wall.Height; y += tile {
		for x := wall.X; x < wall.X+wall.Width; x += tile {
			result = append(result, DrawCommand{Text: "#", X: int(x), Y: int(y)})
		}
	}
	return result
}

var walls = [...]playdate.Rect{{X: 0, Y: 0, Width: 400, Height: 8}, {X: 0, Y: 232, Width: 400, Height: 8}, {X: 0, Y: 0, Width: 8, Height: 240}, {X: 392, Y: 0, Width: 8, Height: 240}, {X: 176, Y: 64, Width: 48, Height: 112}}
var collectibles = [...]playdate.Rect{{X: 40, Y: 40, Width: 12, Height: 12}, {X: 340, Y: 40, Width: 12, Height: 12}, {X: 340, Y: 188, Width: 12, Height: 12}}

// InitialState returns the acceptance scene's canonical reset state.
func InitialState() State {
	return State{Player: playdate.Rect{X: 24, Y: 112, Width: playerSize, Height: playerSize}}
}

// Step applies axis-separated wall sliding and collectible overlaps.
func Step(state State, frame Frame) State {
	if frame.Reset {
		return InitialState()
	}
	next := state.Player
	next.X = moveX(next, frame.DX)
	next.Y = moveY(next, frame.DY)
	state.Player = next
	for index, item := range collectibles {
		bit := uint8(1 << index)
		if state.Collected&bit == 0 && overlaps(next, item) {
			state.Collected |= bit
			state.Score++
		}
	}
	return state
}

func moveX(rect playdate.Rect, delta float32) float32 {
	goal := rect.X + delta
	for _, wall := range walls {
		vertical := rect.Y < wall.Y+wall.Height && rect.Y+rect.Height > wall.Y
		if !vertical {
			continue
		}
		if delta > 0 && rect.X+rect.Width <= wall.X && goal+rect.Width > wall.X {
			goal = wall.X - rect.Width
		}
		if delta < 0 && rect.X >= wall.X+wall.Width && goal < wall.X+wall.Width {
			goal = wall.X + wall.Width
		}
	}
	return goal
}

func moveY(rect playdate.Rect, delta float32) float32 {
	goal := rect.Y + delta
	for _, wall := range walls {
		horizontal := rect.X < wall.X+wall.Width && rect.X+rect.Width > wall.X
		if !horizontal {
			continue
		}
		if delta > 0 && rect.Y+rect.Height <= wall.Y && goal+rect.Height > wall.Y {
			goal = wall.Y - rect.Height
		}
		if delta < 0 && rect.Y >= wall.Y+wall.Height && goal < wall.Y+wall.Height {
			goal = wall.Y + wall.Height
		}
	}
	return goal
}

func hitsAny(rect playdate.Rect, obstacles []playdate.Rect) bool {
	for _, obstacle := range obstacles {
		if overlaps(rect, obstacle) {
			return true
		}
	}
	return false
}
func overlaps(a, b playdate.Rect) bool {
	return a.X < b.X+b.Width && a.X+a.Width > b.X && a.Y < b.Y+b.Height && a.Y+a.Height > b.Y
}

type game struct{ state State }

// New creates the P2.2 acceptance game.
func New() playdate.Game                  { return &game{state: InitialState()} }
func (*game) Init(playdate.Context) error { return nil }
func (g *game) Update(context playdate.Context) (bool, error) {
	input := context.Input()
	dx, dy := float32(0), float32(0)
	if input.Buttons.Has(playdate.ButtonLeft) {
		dx--
	}
	if input.Buttons.Has(playdate.ButtonRight) {
		dx++
	}
	if input.Buttons.Has(playdate.ButtonUp) {
		dy--
	}
	if input.Buttons.Has(playdate.ButtonDown) {
		dy++
	}
	g.state = Step(g.state, Frame{DX: dx * 3, DY: dy * 3, Reset: input.Pressed.Has(playdate.ButtonB)})
	context.Clear()
	for _, command := range DrawPlan(g.state) {
		context.DrawText(command.Text, command.X, command.Y)
	}
	return true, nil
}
