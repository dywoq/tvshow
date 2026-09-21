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
	Name       string
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

var (
	// Cache structures
	// imageFileCache caches decoded image.Image by file path.
	imageFileCache = map[string]image.Image{}

	// tilesetLayerCache caches rendered tileset layer *ebiten.Image.
	// Key is formed by tileset layer properties and room dimensions.
	tilesetLayerCache = map[string]*ebiten.Image{}

	// subSpriteCache caches object sub-sprite *ebiten.Image.
	// Key is formed by sprite sheet path, sub-sprite dimensions, and sub-sprite index.
	subSpriteCache = map[string]*ebiten.Image{}

	// spriteCache caches *ebiten.Image for standalone room Sprites.
	// Key is the image pointer or sprite path.
	spriteCache = map[image.Image]*ebiten.Image{}
)

// ClearCache clears all cached images.
func ClearCache() {
	mu.Lock()
	defer mu.Unlock()
	imageFileCache = map[string]image.Image{}
	tilesetLayerCache = map[string]*ebiten.Image{}
	subSpriteCache = map[string]*ebiten.Image{}
	spriteCache = map[image.Image]*ebiten.Image{}
}

// loadImageFromFile opens and decodes an image, or returns a cached version.
// Callers must hold mu or use it safely.
func loadImageFromFile(filepath string) (image.Image, error) {
	if img, ok := imageFileCache[filepath]; ok {
		return img, nil
	}
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	imageFileCache[filepath] = img
	return img, nil
}

// AddSprite opens a sprite at the provided filepath and decodes it
// using the [image.Decode] method, and stores a [Sprite] instance in
// the current room's sprites slice.
func AddSprite(name, filepath string, x, y int) error {
	mu.Lock()
	defer mu.Unlock()
	r, ok := rooms[currentRoom]
	if !ok || r == nil {
		return fmt.Errorf("room %q does not exist", currentRoom)
	}
	img, err := loadImageFromFile(filepath)
	if err != nil {
		return err
	}
	r.Sprites = append(r.Sprites, &Sprite{
		Name:       name,
		SpritePath: filepath,
		Image:      img,
		X:          x,
		Y:          y,
	})
	return nil
}

func Painter(screen *ebiten.Image) {
	mu.Lock()
	defer mu.Unlock()

	r, ok := rooms[currentRoom]
	if !ok || r == nil {
		return
	}

	for i := range r.Data.TilesetLayers {
		tileset := &r.Data.TilesetLayers[i]
		cacheKey := fmt.Sprintf("%s:%d:%d:%d:%d:%d:%v",
			tileset.TilesetPath, tileset.TileWidth, tileset.TileHeight,
			r.Data.Width, r.Data.Height, len(tileset.Tiles), tileset.Tiles)

		ebImg, exists := tilesetLayerCache[cacheKey]
		if !exists {
			img, err := editor.RenderTilesetLayer(tileset, r.Data.Width, r.Data.Height, "./")
			if err != nil {
				continue
			}
			ebImg = ebiten.NewImageFromImage(img)
			tilesetLayerCache[cacheKey] = ebImg
		}
		screen.DrawImage(ebImg, nil)
	}

	for _, obj := range r.Objects {
		if obj == nil || obj.Data == nil {
			continue
		}
		if obj.Data.SpriteSheet == "" {
			continue
		}

		cacheKey := fmt.Sprintf("%s:%d:%d:%d",
			obj.Data.SpriteSheet, obj.Data.SubSpriteWidth, obj.Data.SubSpriteHeight, obj.CurrentSubSpriteIndex)

		ebSubImg, exists := subSpriteCache[cacheKey]
		if !exists {
			subImg, err := editor.LoadSubSprite(obj.Data, obj.CurrentSubSpriteIndex, "./")
			if err != nil {
				continue
			}
			ebSubImg = ebiten.NewImageFromImage(subImg)
			subSpriteCache[cacheKey] = ebSubImg
		}

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(obj.Data.Coordinates.X), float64(obj.Data.Coordinates.Y))
		screen.DrawImage(ebSubImg, op)
	}

	for _, spr := range r.Sprites {
		if spr.Image == nil {
			continue
		}
		if len(spr.SpritePath) == 0 {
			continue
		}

		ebSprImg, exists := spriteCache[spr.Image]
		if !exists {
			ebSprImg = ebiten.NewImageFromImage(spr.Image)
			spriteCache[spr.Image] = ebSprImg
		}

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(spr.X), float64(spr.Y))
		screen.DrawImage(ebSprImg, op)
	}
}
