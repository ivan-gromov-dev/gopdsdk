package scoreboards

import (
	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	"testing"
)

type context struct{ playdate.Context }

func (context) GetScoreboards(callback func(playdate.BoardsList, error)) error {
	callback(playdate.BoardsList{Boards: []playdate.Board{{ID: boardID}}}, nil)
	return nil
}
func (context) AddScore(string, uint32, func(playdate.Score, error)) error { return nil }
func (context) GetPersonalBest(string, func(playdate.Score, error)) error  { return nil }
func (context) GetScores(string, func(playdate.ScoresList, error)) error   { return nil }
func TestInitRequestsBoards(t *testing.T) {
	game := New().(*game)
	if err := game.Init(context{}); err != nil {
		t.Fatal(err)
	}
	if game.status != "BOARDS: 1" {
		t.Fatalf("status = %q", game.status)
	}
}
