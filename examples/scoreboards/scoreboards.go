// Package scoreboards exercises the P11.2 optional online-scoreboards contract.
package scoreboards

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const boardID = "highscores"

type game struct {
	service playdate.Scoreboards
	status  string
	value   uint32
}

// New creates the P11.2 scoreboards acceptance scene. Its configured-board and
// online-service acceptance remain unverified.
func New() playdate.Game { return &game{status: "REQUESTING BOARDS", value: 100} }

func (game *game) Init(context playdate.Context) error {
	var ok bool
	game.service, ok = any(context).(playdate.Scoreboards)
	if !ok {
		game.status = "FAIL: SCOREBOARDS"
		return nil
	}
	return game.service.GetScoreboards(func(list playdate.BoardsList, err error) {
		if err != nil {
			game.status = "BOARDS: " + err.Error()
			return
		}
		game.status = "BOARDS: " + strconv.Itoa(len(list.Boards))
	})
}

func (game *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P11.2 SCOREBOARDS", 12, 20)
	context.DrawText(game.status, 12, 60)
	context.DrawText("A: ADD  B: PERSONAL BEST", 12, 180)
	context.DrawText("DOWN: LIST SCORES", 12, 200)
	if game.service == nil {
		return true, nil
	}
	if context.Input().Pressed.Has(playdate.ButtonA) {
		game.value++
		err := game.service.AddScore(boardID, game.value, func(score playdate.Score, callbackErr error) {
			if callbackErr != nil {
				game.status = "ADD: " + callbackErr.Error()
				return
			}
			game.status = "RANK " + strconv.FormatUint(uint64(score.Rank), 10)
		})
		if err != nil {
			game.status = "ADD: " + err.Error()
		}
	}
	if context.Input().Pressed.Has(playdate.ButtonB) {
		err := game.service.GetPersonalBest(boardID, func(score playdate.Score, callbackErr error) {
			if callbackErr != nil {
				game.status = "BEST: " + callbackErr.Error()
				return
			}
			game.status = "BEST " + strconv.FormatUint(uint64(score.Value), 10)
		})
		if err != nil {
			game.status = "BEST: " + err.Error()
		}
	}
	if context.Input().Pressed.Has(playdate.ButtonDown) {
		err := game.service.GetScores(boardID, func(list playdate.ScoresList, callbackErr error) {
			if callbackErr != nil {
				game.status = "SCORES: " + callbackErr.Error()
				return
			}
			game.status = "SCORES " + strconv.Itoa(len(list.Scores))
		})
		if err != nil {
			game.status = "SCORES: " + err.Error()
		}
	}
	return true, nil
}
