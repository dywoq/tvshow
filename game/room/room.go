package room

import (
	"fmt"
	"sync"
	"tvshow/game/editor"

	"github.com/hajimehoshi/ebiten/v2"
)

// Object represents a room object instance with its runtime state.
type Object struct {
	Data                  *editor.Object
	CurrentSubSpriteIndex int
}

// Room consists of the room's information.
type Room struct {
	Data    *editor.Room
	Objects []*Object
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
	r, ok := rooms[name]
	if !ok {
		return fmt.Errorf("room %q does not exist", name)
	}
	currentRoom = name

	// Gather all objects from all object layers into r.Objects
	r.Objects = nil
	if r.Data != nil {
		for i := range r.Data.ObjectLayers {
			for j := range r.Data.ObjectLayers[i].Objects {
				r.Objects = append(r.Objects, &Object{
					Data:                  &r.Data.ObjectLayers[i].Objects[j],
					CurrentSubSpriteIndex: 0,
				})
			}
		}
	}

	return nil
}

// GetCurrentRoom returns the current room data.
// Returns an error if it does not exist.
func GetCurrentRoom() (*Room, error) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := rooms[currentRoom]; !ok {
		return nil, fmt.Errorf("room %q does not exist", currentRoom)
	}
	return rooms[currentRoom], nil
}

func Painter(screen *ebiten.Image) {
	room, err := GetCurrentRoom()
	if err != nil {
		return
	}
	for _, tileset := range room.Data.TilesetLayers {
		img, err := editor.RenderTilesetLayer(&tileset, room.Data.Width, room.Data.Height, "./")
		if err != nil {
			continue
		}
		screen.DrawImage(ebiten.NewImageFromImage(img), nil)
	}

	for _, obj := range room.Objects {
		if obj == nil || obj.Data == nil {
			continue
		}
		if obj.Data.SpriteSheet == "" {
			continue
		}
		subImg, err := editor.LoadSubSprite(obj.Data, obj.CurrentSubSpriteIndex, "./")
		if err != nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(obj.Data.Coordinates.X), float64(obj.Data.Coordinates.Y))
		screen.DrawImage(ebiten.NewImageFromImage(subImg), op)
	}
}
