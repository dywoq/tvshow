package program

import (
	"fmt"
	"os"
	"tvshow/game/room"
)

// ProvideFunctionality registers and exposes a set of built-in APIs needed by the game program
// to function properly.
func ProvideFunctionality() {
	// Printing
	interpret.RegisterFunction("__stdout", func(v any) {
		fmt.Printf("%v\n", v)
	})
	interpret.RegisterFunction("__stderr", func(v any) {
		fmt.Fprintf(os.Stderr, "%v\n", v)
	})

	// Change state
	interpret.RegisterFunction("__terminate", func() {
		state = StateTerminated
	})

	// Room functionality
	interpret.RegisterFunction("__room_add", func(name string, filepath string) bool {
		err := room.Add(name, filepath)
		return err == nil
	})
}
