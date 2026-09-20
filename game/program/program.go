package program

import (
	"os"
	"sync"
	"tvshow/game/runtime"
	"tvshow/lang/interpreter"
)

type State int

var (
	mu        sync.Mutex
	interpret *interpreter.Interpreter
	state     State = StateReady
)

const (
	StateActive State = iota
	StateReady
	StateTerminated
)

// HasState checks whether the current state is s.
func HasState(s State) bool {
	mu.Lock()
	defer mu.Unlock()
	return state == s
}

func Initializer() error {
	mu.Lock()
	defer mu.Unlock()
	content, err := os.ReadFile("./game.scb")
	if err != nil {
		return err
	}
	interpret, err = interpreter.NewFromBinary(content)
	if err != nil {
		return err
	}
	ProvideFunctionality()
	
	runtime.SpawnTask(&runtime.Task{
		Func:     Task,
		Status:   runtime.TaskStatusReady,
		Priority: runtime.TaskPriorityHigh,
	})
	state = StateActive

	return nil
}

func Task(t *runtime.Task) (runtime.TaskAction, error) {
	if state == StateTerminated {
		return runtime.TaskActionFinish, nil
	}
	_, err := interpret.Run("game_frame")
	if err != nil {
		return runtime.TaskActionFinish, err
	}
	return runtime.TaskActionYield, nil
}
