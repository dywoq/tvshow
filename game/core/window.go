package core

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// Window consists of the game window configuration settings.
// It provides a way to run the game.
type Window struct {
	Width     int
	Height    int
	Title     string
	Executors []Executor
}

// Executor is a function that is executed every frame.
// It can be used to run guest program code, calculate coordinates and etc.
// related to game logic.
type Executor func() error

type ebitenWindow struct {
	w *Window
}

func (e *ebitenWindow) Update() error {
	for _, ex := range e.w.Executors {
		err := ex()
		if err != nil {
			return fmt.Errorf("one of the executors failed: %v", err)
		}
	}
	return nil
}

func (e *ebitenWindow) Draw(screen *ebiten.Image) {}

func (e *ebitenWindow) Layout(int, int) (int, int) {
	return e.w.Width, e.w.Height
}

// Run sets the window settings up and starts the game.
func (w *Window) Run() error {
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle(w.Title)
	return ebiten.RunGame(&ebitenWindow{
		w: w,
	})
}
