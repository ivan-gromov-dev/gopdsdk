// Package schedule demonstrates bounded incremental work across update frames.
package schedule

import (
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	pdschedule "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule"
)

const (
	totalItems    = 120
	itemsPerStep  = 3
	stepsPerFrame = 2
	taskCount     = 4
	itemsPerTask  = totalItems / taskCount
	totalSteps    = totalItems / itemsPerStep
	activeFrames  = totalSteps / stepsPerFrame
)

type game struct {
	scheduler       *pdschedule.Scheduler
	taskItems       [taskCount]int
	processed       int
	frames          int
	executedSteps   int
	peakSteps       int
	completionFrame int
	violated        bool
	finished        bool
}

// New creates the incremental-work acceptance game.
func New() playdate.Game { return &game{} }

func (game *game) Init(context playdate.Context) error {
	scheduler, err := pdschedule.New(context, pdschedule.Config{
		Capacity:      taskCount,
		StepsPerFrame: stepsPerFrame,
	})
	if err != nil {
		return err
	}
	game.scheduler = scheduler
	for task := range taskCount {
		task := task
		if _, err = scheduler.Schedule(func() pdschedule.Action { return game.processStep(task) }); err != nil {
			scheduler.Terminate()
			return err
		}
	}
	return nil
}

func (game *game) processStep(task int) pdschedule.Action {
	remaining := itemsPerTask - game.taskItems[task]
	if remaining > itemsPerStep {
		remaining = itemsPerStep
	}
	game.taskItems[task] += remaining
	game.processed += remaining
	if game.taskItems[task] == itemsPerTask {
		return pdschedule.Complete()
	}
	return pdschedule.Yield()
}

func (game *game) Update(context playdate.Context) (bool, error) {
	frame, err := game.scheduler.Update()
	if err != nil {
		return false, err
	}
	game.frames++
	game.executedSteps += frame.Steps
	if frame.Steps > game.peakSteps {
		game.peakSteps = frame.Steps
	}
	if frame.Steps > stepsPerFrame {
		game.violated = true
	}
	if frame.Pending == 0 && !game.finished {
		game.finished = true
		game.completionFrame = game.frames
	}
	context.Clear()
	context.DrawText("P12.1 device step proof", 12, 8)
	context.DrawText("Items "+ratio(game.processed, totalItems), 12, 30)
	context.DrawText("Last steps "+ratio(frame.Steps, stepsPerFrame), 12, 52)
	context.DrawText("Peak steps "+ratio(game.peakSteps, stepsPerFrame)+" "+pass(game.peakSteps == stepsPerFrame && !game.violated), 12, 74)
	context.DrawText("Step count "+ratio(game.executedSteps, totalSteps)+" "+pass(game.finished && game.executedSteps == totalSteps), 12, 96)
	context.DrawText("Complete frame "+ratio(game.completionFrame, activeFrames)+" "+pass(game.finished && game.completionFrame == activeFrames), 12, 118)
	context.DrawText("Tasks "+taskLine(game.taskItems), 12, 140)
	context.DrawText("Frame "+strconv.Itoa(game.frames)+" State "+pass(game.proofPassed()), 12, 162)
	return true, nil
}

func (game *game) proofPassed() bool {
	if !game.finished || game.processed != totalItems || game.executedSteps != totalSteps || game.peakSteps != stepsPerFrame || game.completionFrame != activeFrames || game.violated {
		return false
	}
	for _, processed := range game.taskItems {
		if processed != itemsPerTask {
			return false
		}
	}
	return true
}

func ratio(value, expected int) string {
	return strconv.Itoa(value) + "/" + strconv.Itoa(expected)
}

func pass(ok bool) string {
	if ok {
		return "PASS"
	}
	return "----"
}

func taskLine(tasks [taskCount]int) string {
	return strconv.Itoa(tasks[0]) + "/" + strconv.Itoa(tasks[1]) + "/" + strconv.Itoa(tasks[2]) + "/" + strconv.Itoa(tasks[3])
}

func (game *game) HandleLifecycle(_ playdate.Context, event playdate.LifecycleEvent) error {
	if event == playdate.LifecycleTerminate && game.scheduler != nil {
		game.scheduler.Terminate()
	}
	return nil
}
