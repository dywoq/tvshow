package room

import (
	"fmt"
	"sync"
	"tvshow/game/editor"
)

// Room consists of the room's information.
type Room struct {
	Data *editor.Room
}

var (
	mu          sync.Mutex
	rooms       = map[string]*Room{}
	currentRoom = ""
)

// Add loads the room data into memory and saves it in the underlying map.
// Returns an error if a room with the provided name exists.
func Add(name string, filepath string) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := rooms[name]; ok {
		return fmt.Errorf("room %q already exists", name)
	}
	d, err := editor.LoadRoomFromFile(filepath)
	if err != nil {
		return err
	}
	rooms[name] = &Room{
		Data: d,
	}
	return nil
}

// SetCurrentRoom sets the current room to the one having the provided name.
// Returns an error if it does not exist.
func SetCurrentRoom(name string) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := rooms[name]; ok {
		return fmt.Errorf("room %q does not exist", name)
	}
	currentRoom = name
	return nil
}
