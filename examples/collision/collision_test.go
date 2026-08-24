package collision

import (
	"reflect"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestPointRectAndSpriteQueriesFilterTypeAndGroup(t *testing.T) {
	objects := []Object{{Rect: collectibles[0], Type: CollectibleType, Group: collectibleGroup}, {Rect: walls[0], Type: WallType, Group: wallGroup}}
	if got := QueryPoint(objects, playdate.Point{X: 42, Y: 42}, Filter{Types: CollectibleType, Groups: collectibleGroup}); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("point query = %v", got)
	}
	if got := QueryRect(objects, playdate.Rect{X: 0, Y: 0, Width: 50, Height: 50}, Filter{Types: WallType}); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("rect query = %v", got)
	}
	if got := QuerySprite(objects, collectibles[0], Filter{Groups: wallGroup}); len(got) != 0 {
		t.Fatalf("sprite group filter = %v", got)
	}
}

func TestAcceptancePlayerWallsCollectiblesScoreAndReset(t *testing.T) {
	state := InitialState()
	state = Step(state, Frame{DX: -40})
	if state.Player.X != 8 {
		t.Fatalf("player crossed wall: %+v", state.Player)
	}
	state = InitialState()
	state = Step(state, Frame{DX: 16, DY: -72})
	if state.Score != 1 || state.Collected != 1 {
		t.Fatalf("collect = %+v", state)
	}
	state = Step(state, Frame{})
	if state.Score != 1 {
		t.Fatalf("collectible scored twice: %+v", state)
	}
	state = Step(state, Frame{Reset: true})
	if state != InitialState() {
		t.Fatalf("reset = %+v", state)
	}
}

func TestPlanSlidesAlongInteriorWallDeterministically(t *testing.T) {
	state := InitialState()
	state.Player.X = 160
	state.Player.Y = 80
	got := Step(state, Frame{DX: 20, DY: 10})
	if got.Player.X != 160 || got.Player.Y != 90 {
		t.Fatalf("slide = %+v", got.Player)
	}
}

func TestDrawPlanShowsSceneScoreAndCollectedState(t *testing.T) {
	state := InitialState()
	state.Score = 1
	state.Collected = 1
	plan := DrawPlan(state)
	if plan[0].Text != "P2.2 collision  score:1  B:reset" || plan[len(plan)-1].Text != "@" {
		t.Fatalf("draw plan = %#v", plan)
	}
	stars := 0
	hashes := 0
	for _, command := range plan {
		if command.Text == "*" {
			stars++
		}
		if command.Text == "#" {
			hashes++
		}
	}
	if stars != 2 {
		t.Fatalf("collectibles drawn = %d", stars)
	}
	if hashes != 244 {
		t.Fatalf("wall tiles drawn = %d, want 244", hashes)
	}
}
