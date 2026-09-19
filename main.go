package main

import (
	"tvshow/game/core"
)

func main() {
	w := &core.Window{
		Width:  640,
		Height: 480,
		Title:  "ТВ Шоу 2",
	}
	if err := w.Run(); err != nil {
		panic(err)
	}
}
