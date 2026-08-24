// Package defercleanup demonstrates normal-return defer cleanup on Playdate.
package defercleanup

import (
	"runtime"
	"strconv"
	"time"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	repeatedCount  = 8
	soakDurationMS = 60 * 1000
	heapGrowthMax  = 32 * 1024
)

type resource struct {
	closed *int
}

func (resource resource) Close() { *resource.closed++ }

type game struct {
	frames       uint32
	startMS      uint32
	started      bool
	cleanups     int
	semanticsOK  bool
	timeOK       bool
	baselineHeap uint64
	maxHeap      uint64
	soakComplete bool
	memoryOK     bool
}

// New creates the normal-return defer acceptance game.
func New() playdate.Game { return &game{} }

func (game *game) Init(playdate.Context) error {
	game.semanticsOK = verifySemantics()
	formatted, err := durationFixture("1h2m3.004s")
	game.timeOK = err == nil && formatted == "1h2m3.004s"
	return nil
}

func (game *game) Update(context playdate.Context) (bool, error) {
	now := context.CurrentTimeMilliseconds()
	if !game.started {
		game.started = true
		game.startMS = now
	}
	before := game.cleanups
	repeatedCleanup(repeatedCount, &game.cleanups)
	game.frames++
	if game.cleanups-before != repeatedCount {
		game.semanticsOK = false
	}
	if game.frames%30 == 0 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if game.baselineHeap == 0 {
			game.baselineHeap = stats.HeapAlloc
		}
		if stats.HeapAlloc > game.maxHeap {
			game.maxHeap = stats.HeapAlloc
		}
		game.memoryOK = game.maxHeap-game.baselineHeap <= heapGrowthMax
	}
	game.soakComplete = now-game.startMS >= soakDurationMS

	context.Clear()
	context.DrawText("P12.2 defer proof", 12, 8)
	context.DrawText("Semantics "+pass(game.semanticsOK), 12, 30)
	context.DrawText("Cleanup "+strconv.Itoa(game.cleanups)+" "+pass(game.cleanups == int(game.frames)*repeatedCount), 12, 52)
	context.DrawText("Duration "+pass(game.timeOK), 12, 74)
	context.DrawText("Memory "+pass(game.memoryOK), 12, 96)
	context.DrawText("Soak "+pass(game.soakComplete), 12, 118)
	return true, nil
}

func earlyReturn(cleaned *int, value int) (result int) {
	defer resource{closed: cleaned}.Close()
	result = value
	defer func() { result++ }()
	return
}

func argumentEvaluation(values *[]int, value int) {
	defer func(captured int) { *values = append(*values, captured) }(value)
	value++
}

func lifo(values *[]int) {
	defer func() { *values = append(*values, 1) }()
	defer func() { *values = append(*values, 2) }()
}

func repeatedCleanup(count int, cleaned *int) {
	for range count {
		defer resource{closed: cleaned}.Close()
	}
}

func durationFixture(text string) (string, error) {
	duration, err := time.ParseDuration(text)
	if err != nil {
		return "", err
	}
	return duration.String(), nil
}

func verifySemantics() bool {
	cleaned := 0
	if earlyReturn(&cleaned, 41) != 42 || cleaned != 1 {
		return false
	}
	values := make([]int, 0, 3)
	argumentEvaluation(&values, 7)
	lifo(&values)
	return len(values) == 3 && values[0] == 7 && values[1] == 2 && values[2] == 1
}

func pass(ok bool) string {
	if ok {
		return "PASS"
	}
	return "----"
}
