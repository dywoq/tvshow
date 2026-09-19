package main

import (
	"fmt"
	"os"
	"tvshow/game"
)

func main() {
	err := game.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start the game: %v\n", err)
	}
}
