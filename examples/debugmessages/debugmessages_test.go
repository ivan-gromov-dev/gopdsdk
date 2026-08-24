package debugmessages

import (
	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	"testing"
)

type messages struct {
	playdate.Context
	items []string
}

func (m *messages) PollDebugMessage() (string, bool) {
	if len(m.items) == 0 {
		return "", false
	}
	value := m.items[0]
	m.items = m.items[1:]
	return value, true
}
func TestUpdateDrainsMessages(t *testing.T) {
	context := &messages{items: []string{"one", "two"}}
	game := New().(*game)
	if err := game.Init(context); err != nil {
		t.Fatal(err)
	}
	game.messages = context
	for {
		_, ok := game.messages.PollDebugMessage()
		if !ok {
			break
		}
		game.count++
	}
	if game.count != 2 {
		t.Fatalf("count = %d", game.count)
	}
}
