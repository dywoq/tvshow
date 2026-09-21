package room

import (
	"fmt"
	"image"
	"os"
	"sync"
	"tvshow/game/editor"

	"github.com/hajimehoshi/ebiten/v2"
)

// Object represents a room object instance with its runtime state.
type Object struct {
	Data                  *editor.Object
	CurrentSubSpriteIndex int
}

// Sprite represents a room independent sprite that does not belong to an object.
type Sprite struct {
	SpritePath string
	Image      image.Image
	X          int
	Y          int
}

// Room consists of the room's information.
type Room struct {
	Data    *editor.Room
	Objects []*Object
	Sprites []*Sprite
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
	r.Sprites = []*Sprite{}
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

// GetCurrentRoomName returns an empty string or a current room name.
func GetCurrentRoomName() string {
	mu.Lock()
	defer mu.Unlock()
	return currentRoom
}

// AddSprite opens a sprite at the provided filepath and decodes it
// using the [image.Decode] method, and stores a [Sprite] instance in
// the current room's sprites slice.
func AddSprite(filepath string, x, y int) error {
	mu.Lock()
	defer mu.Unlock()
	r := rooms[currentRoom]
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	r.Sprites = append(r.Sprites, &Sprite{
		SpritePath: filepath,
		Image:      img,
		X:          x,
		Y:          y,
	})
	return nil
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

	for _, spr := range room.Sprites {
		if spr.Image == nil {
			continue
		}
		if len(spr.SpritePath) == 0 {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(spr.X), float64(spr.Y))
		screen.DrawImage(ebiten.NewImageFromImage(spr.Image), op)
	}
}
