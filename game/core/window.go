package core

import (
	"errors"
	"fmt"
	"tvshow/game/runtime"

	"github.com/hajimehoshi/ebiten/v2"
)

type windowState int

// Window consists of the game window configuration settings.
// It provides a way to run the game.
type Window struct {
	Width        int
	Height       int
	Title        string
	Executors    []Executor
	Initializers []Initializer
	Painters     []Painter
	Cleaners     []Cleaner
	state        windowState
}

// Executor is a function that is executed every frame.
// It can be used to run guest program code, calculate coordinates and etc.
// related to game logic.
type Executor func() error

// Painter is a function executed internally by [Window] structure.
// It is responsible for drawing something into screen.
type Painter func(screen *ebiten.Image)

// Initializer is a function that is executed before the game.
// It is used to initialize game's critical components.
type Initializer func() error

// Cleaner is a function that is executed after the game.
// It is used to clean resources of game's critical components.
type Cleaner func() error

type ebitenWindow struct {
	w *Window
}

const (
	windowStateReady windowState = iota
	windowStateActive
	windowStateClosed
)

func (e *ebitenWindow) Update() error {
	if e.w.state == windowStateClosed {
		return errors.New("window closed")
	}
	for _, ex := range e.w.Executors {
		err := ex()
		if err != nil {
			return fmt.Errorf("one of the executors failed: %v", err)
		}
	}
	return nil
}

func (e *ebitenWindow) Draw(screen *ebiten.Image) {
	for _, p := range e.w.Painters {
		p(screen)
	}
}

func (e *ebitenWindow) Layout(int, int) (int, int) {
	return e.w.Width, e.w.Height
}

// Run sets the window settings up and starts the game.
func (w *Window) Run() error {
	w.state = windowStateActive
	defer func() {
		w.state = windowStateReady
	}()

	for _, init := range w.Initializers {
		err := init()
		if err != nil {
			return fmt.Errorf("one of the initializers failed: %v", err)
		}
	}

	// Set the runtime signal up. It is supposed to change the window's state.
	// The ebitenWindow.Update method returns an error if the state is windowStateClosed.
	err := runtime.AddSignal("window_close", []runtime.SignalFunction{
		func() error {
			w.state = windowStateClosed
			return nil
		},
	})
	if err != nil {
		return err
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle(w.Title)
	ebiten.SetTPS(30)
	err = ebiten.RunGame(&ebitenWindow{
		w: w,
	})
	if err != nil {
		return fmt.Errorf("running the game failed: %v", err)
	}
	for _, clean := range w.Cleaners {
		err := clean()
		if err != nil {
			return fmt.Errorf("one of the cleaners failed: %v", err)
		}
	}
	return nil
}
