package program

import (
	"os"
	"sync"
	"tvshow/game/runtime"
	"tvshow/lang/interpreter"
)

var (
	mu        sync.Mutex
	interpret *interpreter.Interpreter
)

func Initializer() error {
	mu.Lock()
	defer mu.Unlock()
	content, err := os.ReadFile("game.scb")
	if err != nil {
		return err
	}
	interpret, err = interpreter.NewFromBinary(content)
	if err != nil {
		return err
	}
	return nil
}

func Task(t *runtime.Task) (runtime.TaskAction, error) {
	_, err := interpret.Run("game_frame")
	if err != nil {
		return runtime.TaskActionFinish, err
	}
	return runtime.TaskActionYield, nil
}
