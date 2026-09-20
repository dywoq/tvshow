package program

import (
	"fmt"
	"os"
	"slices"
	"tvshow/game/keyboard"
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
	interpret.RegisterFunction("__room_get_object_x", func(ttype string) int {
		room, err := room.GetCurrentRoom()
		if err != nil {
			return -1
		}
		for _, obj := range room.Objects {
			if obj.Data.Type == ttype {
				return obj.Data.Coordinates.X
			}
		}
		return -1
	})
	interpret.RegisterFunction("__room_get_object_y", func(ttype string) int {
		room, err := room.GetCurrentRoom()
		if err != nil {
			return -1
		}
		for _, obj := range room.Objects {
			if obj.Data.Type == ttype {
				return obj.Data.Coordinates.Y
			}
		}
		return -1
	})
	interpret.RegisterFunction("__room_set_object_subsprite_index", func(ttype string, subspriteIndex int) bool {
		room, err := room.GetCurrentRoom()
		if err != nil {
			return false
		}
		for _, obj := range room.Objects {
			if obj.Data.Type == ttype {
				obj.CurrentSubSpriteIndex = subspriteIndex
				return true
			}
		}
		return false
	})
	interpret.RegisterFunction("__room_get_object_subsprite_count", func(ttype string) int {
		room, err := room.GetCurrentRoom()
		if err != nil {
			return -1
		}
		for _, obj := range room.Objects {
			if obj.Data.Type == ttype {
				return obj.Data.SubSpritesTotalCount
			}
		}
		return -1
	})
	interpret.RegisterFunction("__room_object_exists", func(ttype string) bool {
		room, err := room.GetCurrentRoom()
		if err != nil {
			return false
		}
		for _, obj := range room.Objects {
			if obj.Data.Type == ttype {
				return true
			}
		}
		return false
	})
	interpret.RegisterFunction("__room_object_has_attributes", func(ttype string, attributes []string) bool {
		if len(attributes) == 0 {
			return false
		}
		room, err := room.GetCurrentRoom()
		if err != nil {
			return false
		}
		for _, obj := range room.Objects {
			if obj.Data.Type == ttype {
				satisfiedCount := 0
				for _, attribute := range attributes {
					if slices.Contains(obj.Data.Attributes, attribute) {
						satisfiedCount++
					}
				}
				return satisfiedCount == len(attributes)
			}
		}
		return false
	})

	// Keyboard functionality
	interpret.RegisterFunction("__key_pressed", func(key string) bool {
		return keyboard.Pressed(key)
	})
	interpret.RegisterFunction("__key_just_pressed", func(key string) bool {
		return keyboard.JustPressed(key)
	})
	interpret.RegisterFunction("__key_just_released", func(key string) bool {
		return keyboard.JustReleased(key)
	})
}
