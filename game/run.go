package game

import (
	"fmt"
	"tvshow/game/core"
	"tvshow/game/runtime"
)

// Run configures the game's window, its executors and starts the game.
func Run() error {
	w := &core.Window{
		Width:  640,
		Height: 480,
		Title:  "ТВ Шоу 2",
		Executors: []core.Executor{
			runtime.TaskExecutor,
		},
	}
	if err := w.Run(); err != nil {
		return fmt.Errorf("running the game failed: %v", err)
	}

	return nil
}
