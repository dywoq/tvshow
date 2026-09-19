package core

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// Window consists of the game window configuration settings.
// It provides a way to run the game.
type Window struct {
	Width  int
	Height int
	Title  string
}

type ebitenWindow struct {
	w *Window
}

func (e *ebitenWindow) Update() error {
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
