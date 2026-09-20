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
		return room.Add(name, filepath) == nil
	})
	interpret.RegisterFunction("__room_set_current", func(name string) bool {
		return room.SetCurrentRoom(name) == nil
	})
	interpret.RegisterFunction("__room_set_object_pos", func(ttype string, x, y int) bool {
		room, err := room.GetCurrentRoom()
		if err != nil {
			return false
		}
		for _, obj := range room.Objects {
			if obj.Data.Type == ttype {
				obj.Data.Coordinates.X = x
				obj.Data.Coordinates.Y = y 
				return true
			}
		}
		return false
	})
}
