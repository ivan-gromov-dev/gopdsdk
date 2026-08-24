package runtime

import (
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestScoreboardServiceBoundsAndCopiesCallbacks(t *testing.T) {
	var finish func(playdate.Score, string)
	service := NewScoreboardService(ScoreboardDriver{AddScore: func(board string, value uint32, callback func(playdate.Score, string)) bool {
		if board != "daily" || (value != 42 && value != 44) {
			t.Fatalf("request = %q %d", board, value)
		}
		finish = callback
		return true
	}})
	if err := service.AddScore("daily", 42, func(score playdate.Score, err error) {
		if err != nil || score.Player != "Ada" {
			t.Errorf("callback = %+v, %v", score, err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.AddScore("daily", 43, func(playdate.Score, error) {}); !errors.Is(err, playdate.ErrScoreboardBusy) {
		t.Fatalf("busy error = %v", err)
	}
	finish(playdate.Score{Player: "Ada"}, "")
	if err := service.AddScore("daily", 44, func(playdate.Score, error) {}); err != nil {
		t.Fatalf("request after callback: %v", err)
	}
}

func TestScoreboardServiceValidationAndFailures(t *testing.T) {
	service := NewScoreboardService(ScoreboardDriver{Scores: func(string, func(playdate.ScoresList, string)) bool { return false }})
	if err := service.GetScores("", func(playdate.ScoresList, error) {}); !errors.Is(err, playdate.ErrScoreboardBoardID) {
		t.Fatalf("board error = %v", err)
	}
	if err := service.GetScores("daily", nil); !errors.Is(err, playdate.ErrScoreboardCallback) {
		t.Fatalf("callback error = %v", err)
	}
	if err := service.GetScores("daily", func(playdate.ScoresList, error) {}); !errors.Is(err, playdate.ErrScoreboardRequest) {
		t.Fatalf("request error = %v", err)
	}
}

func TestScoreboardServiceTerminationCancelsPendingCallback(t *testing.T) {
	var finish func(playdate.Score, string)
	called := 0
	service := NewScoreboardService(ScoreboardDriver{AddScore: func(_ string, _ uint32, callback func(playdate.Score, string)) bool {
		finish = callback
		return true
	}})
	if err := service.AddScore("daily", 42, func(playdate.Score, error) { called++ }); err != nil {
		t.Fatal(err)
	}
	service.Terminate()
	finish(playdate.Score{}, "late failure")
	if called != 0 {
		t.Fatalf("callback count after termination = %d", called)
	}
	if err := service.AddScore("daily", 43, func(playdate.Score, error) {}); !errors.Is(err, playdate.ErrScoreboardUnavailable) {
		t.Fatalf("request after termination = %v", err)
	}
}

func TestScoreboardCallbackQueueBoundsDefersAndTerminates(t *testing.T) {
	queue := &ScoreboardCallbackQueue{}
	called := 0
	for range 4 {
		if !queue.Push(func() { called++ }) {
			t.Fatal("queue rejected bounded completion")
		}
	}
	overflowAccepted := queue.Push(func() { called++ })
	if overflowAccepted || called != 0 {
		t.Fatalf("overflow accepted or callback re-entered: accepted=%v called=%d", overflowAccepted, called)
	}
	queue.Drain()
	if called != 4 {
		t.Fatalf("drained callbacks = %d", called)
	}
	if !queue.Push(func() { called++ }) {
		t.Fatal("queue did not accept after drain")
	}
	queue.Terminate()
	queue.Drain()
	if called != 4 || queue.Push(func() { called++ }) {
		t.Fatalf("termination did not suppress callbacks: %d", called)
	}
}

func TestDebugMessageQueueBounds(t *testing.T) {
	queue := NewDebugMessageQueue(2, 4)
	queue.Push("first")
	queue.Push("two")
	queue.Push("last")
	if got, ok := queue.Poll(); !ok || got != "two" {
		t.Fatalf("first poll = %q %v", got, ok)
	}
	if got, ok := queue.Poll(); !ok || got != "last" {
		t.Fatalf("second poll = %q %v", got, ok)
	}
}

type onlineContext struct {
	testContext
	queue *DebugMessageQueue
	add   func(playdate.Score, string)
}

func (c *onlineContext) PollDebugMessage() (string, bool) { return c.queue.Poll() }
func (c *onlineContext) AddScore(_ string, _ uint32, callback func(playdate.Score, error)) error {
	c.add = func(score playdate.Score, message string) {
		var err error
		if message != "" {
			err = playdate.ScoreboardOperationError{Operation: "add score", Message: message}
		}
		callback(score, err)
	}
	return nil
}
func (*onlineContext) GetPersonalBest(string, func(playdate.Score, error)) error { return nil }
func (*onlineContext) GetScoreboards(func(playdate.BoardsList, error)) error     { return nil }
func (*onlineContext) GetScores(string, func(playdate.ScoresList, error)) error  { return nil }

type onlineGame struct {
	messages  []string
	callbacks int
}

func (g *onlineGame) Init(context playdate.Context) error {
	messages := context.(playdate.DebugMessages)
	if value, ok := messages.PollDebugMessage(); ok {
		g.messages = append(g.messages, value)
	}
	return context.(playdate.Scoreboards).AddScore("daily", 7, func(playdate.Score, error) { g.callbacks++ })
}
func (*onlineGame) Update(playdate.Context) (bool, error) { return false, nil }

func TestNewApplicationForwardsOnlineDebugAndSuppressesLateCallback(t *testing.T) {
	context := &onlineContext{queue: NewDebugMessageQueue(4, 32)}
	context.queue.Push("start")
	game := &onlineGame{}
	application, err := NewApplication(game, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if len(game.messages) != 1 || game.messages[0] != "start" {
		t.Fatalf("messages = %v", game.messages)
	}
	context.queue.Push("discard")
	if err := application.Handle(EventTerminate, 0); err != nil {
		t.Fatal(err)
	}
	context.add(playdate.Score{}, "")
	if game.callbacks != 0 {
		t.Fatalf("callbacks after termination = %d", game.callbacks)
	}
	if _, ok := context.queue.Poll(); ok {
		t.Fatal("debug queue not drained")
	}
}
