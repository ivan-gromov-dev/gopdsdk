// Package debugmessages exercises the P4.5 bounded diagnostic-message input.
package debugmessages

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

type game struct {
	messages playdate.DebugMessages
	last     string
	count    int
}

// New creates the P4.5 debug-message acceptance scene. Simulator-console and
// physical-device serial acceptance remain unverified.
func New() playdate.Game { return &game{last: "SEND !msg hello"} }

func (game *game) Init(context playdate.Context) error {
	game.messages, _ = any(context).(playdate.DebugMessages)
	if game.messages == nil {
		game.last = "FAIL: DEBUG MESSAGES"
	}
	return nil
}
func (game *game) Update(context playdate.Context) (bool, error) {
	if game.messages != nil {
		for {
			message, ok := game.messages.PollDebugMessage()
			if !ok {
				break
			}
			game.last = message
			game.count++
		}
	}
	context.Clear()
	context.DrawText("P4.5 DEBUG MESSAGES", 12, 30)
	context.DrawText(game.last, 12, 80)
	return true, nil
}
