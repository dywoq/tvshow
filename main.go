package main

import (
	"fmt"
	"os"
	"tvshow/game"
)

func main() {
	err := game.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
