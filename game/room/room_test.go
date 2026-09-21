package room

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"tvshow/game/editor"
)

func TestRoomObjectsAndSetCurrentRoom(t *testing.T) {
	// Create a temporary directory for room JSON
	tempDir := t.TempDir()

	roomData := &editor.Room{
		Width:  320,
		Height: 240,
		ObjectLayers: []editor.ObjectLayer{
			{
				Name: "Layer 1",
				Objects: []editor.Object{
					{
						Type:        "Player",
						Width:       16,
						Height:      16,
						Coordinates: editor.Coordinates{X: 10, Y: 20},
					},
					{
						Type:        "NPC",
						Width:       16,
						Height:      16,
						Coordinates: editor.Coordinates{X: 30, Y: 40},
					},
				},
			},
			{
				Name: "Layer 2",
				Objects: []editor.Object{
					{
						Type:        "Item",
						Width:       8,
						Height:      8,
						Coordinates: editor.Coordinates{X: 50, Y: 60},
					},
				},
			},
		},
		TilesetLayers: []editor.TilesetLayer{},
	}

	roomFile := tempDir + "/test_room.json"
	if err := editor.SaveRoomToFile(roomData, roomFile); err != nil {
		t.Fatalf("Failed to save test room file: %v", err)
	}

	roomName := "test_room_1"
	if err := Add(roomName, roomFile); err != nil {
		t.Fatalf("Failed to add room: %v", err)
	}

	// Test adding existing room returns error
	if err := Add(roomName, roomFile); err == nil {
		t.Fatalf("Expected error when adding existing room, got nil")
	}

	// Test setting current room
	if err := SetCurrentRoom(roomName); err != nil {
		t.Fatalf("Failed to set current room: %v", err)
	}

	// Get current room
	r, err := GetCurrentRoom()
	if err != nil {
		t.Fatalf("Failed to get current room: %v", err)
	}

	if r.Data.Width != 320 || r.Data.Height != 240 {
		t.Errorf("Expected room dimensions 320x240, got %dx%d", r.Data.Width, r.Data.Height)
	}

	// Check gathered objects
	if len(r.Objects) != 3 {
		t.Fatalf("Expected 3 objects in room, got %d", len(r.Objects))
	}

	expectedTypes := []string{"Player", "NPC", "Item"}
	for i, obj := range r.Objects {
		if obj == nil || obj.Data == nil {
			t.Fatalf("Object at index %d is nil", i)
		}
		if obj.Data.Type != expectedTypes[i] {
			t.Errorf("Expected object %d type %q, got %q", i, expectedTypes[i], obj.Data.Type)
		}
		if obj.CurrentSubSpriteIndex != 0 {
			t.Errorf("Expected initial CurrentSubSpriteIndex 0, got %d", obj.CurrentSubSpriteIndex)
		}
	}

	// Test modifying CurrentSubSpriteIndex
	r.Objects[0].CurrentSubSpriteIndex = 2
	if r.Objects[0].CurrentSubSpriteIndex != 2 {
		t.Errorf("Expected CurrentSubSpriteIndex to be 2, got %d", r.Objects[0].CurrentSubSpriteIndex)
	}

	// Test non-existent room
	if err := SetCurrentRoom("non_existent"); err == nil {
		t.Errorf("Expected error for non-existent room, got nil")
	}
}

func TestPainter(t *testing.T) {
	tempDir := t.TempDir()

	roomData := &editor.Room{
		Width:  100,
		Height: 100,
		ObjectLayers: []editor.ObjectLayer{
			{
				Objects: []editor.Object{
					{
						Type:        "ObjectWithoutSprite",
						Coordinates: editor.Coordinates{X: 5, Y: 5},
					},
				},
			},
		},
	}

	roomFile := tempDir + "/painter_room.json"
	if err := editor.SaveRoomToFile(roomData, roomFile); err != nil {
		t.Fatalf("Failed to save painter room file: %v", err)
	}

	// Register and set current room
	if err := Add("painter_room", roomFile); err != nil {
		t.Fatalf("Failed to add room: %v", err)
	}
	if err := SetCurrentRoom("painter_room"); err != nil {
		t.Fatalf("Failed to set current room: %v", err)
	}

	screen := ebiten.NewImage(100, 100)
	// Painter should run without panic
	Painter(screen)
}

func TestImageCachingAndAddSprite(t *testing.T) {
	tempDir := t.TempDir()
	ClearCache()

	// Create a test PNG image file
	imgFile := tempDir + "/test_sprite.png"
	m := image.NewRGBA(image.Rect(0, 0, 16, 16))
	f, err := os.Create(imgFile)
	if err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}
	if err := png.Encode(f, m); err != nil {
		f.Close()
		t.Fatalf("Failed to encode test image: %v", err)
	}
	f.Close()

	// Setup room
	roomData := &editor.Room{Width: 100, Height: 100}
	roomFile := tempDir + "/cache_room.json"
	if err := editor.SaveRoomToFile(roomData, roomFile); err != nil {
		t.Fatalf("Failed to save room file: %v", err)
	}
	if err := Add("cache_room", roomFile); err != nil {
		t.Fatalf("Failed to add room: %v", err)
	}
	if err := SetCurrentRoom("cache_room"); err != nil {
		t.Fatalf("Failed to set current room: %v", err)
	}

	// Add sprite twice using same filepath
	if err := AddSprite("spr1", imgFile, 10, 10); err != nil {
		t.Fatalf("Failed to add spr1: %v", err)
	}
	if err := AddSprite("spr2", imgFile, 20, 20); err != nil {
		t.Fatalf("Failed to add spr2: %v", err)
	}

	// Delete image file from disk to verify caching avoids file operations
	if err := os.Remove(imgFile); err != nil {
		t.Fatalf("Failed to remove test image file: %v", err)
	}

	// Adding sprite with cached path should still succeed because it hits imageFileCache
	if err := AddSprite("spr3", imgFile, 30, 30); err != nil {
		t.Fatalf("Expected AddSprite to succeed from image cache, got: %v", err)
	}

	// Test Painter execution with cached sprites
	screen := ebiten.NewImage(100, 100)
	Painter(screen)

	// Clear cache and check that AddSprite fails when file is missing
	ClearCache()
	if err := AddSprite("spr4", imgFile, 40, 40); err == nil {
		t.Errorf("Expected AddSprite to fail after cache clear for missing file, got nil")
	}
}

func TestPainterTilesetAndSubSpriteCaching(t *testing.T) {
	tempDir := t.TempDir()
	ClearCache()

	// Create tileset image file
	tilesetPath := tempDir + "/tileset.png"
	tsImg := image.NewRGBA(image.Rect(0, 0, 32, 32))
	f1, err := os.Create(tilesetPath)
	if err != nil {
		t.Fatalf("Failed to create tileset file: %v", err)
	}
	if err := png.Encode(f1, tsImg); err != nil {
		f1.Close()
		t.Fatalf("Failed to encode tileset image: %v", err)
	}
	f1.Close()

	// Create sprite sheet image file
	spriteSheetPath := tempDir + "/spritesheet.png"
	ssImg := image.NewRGBA(image.Rect(0, 0, 32, 32))
	f2, err := os.Create(spriteSheetPath)
	if err != nil {
		t.Fatalf("Failed to create sprite sheet file: %v", err)
	}
	if err := png.Encode(f2, ssImg); err != nil {
		f2.Close()
		t.Fatalf("Failed to encode sprite sheet image: %v", err)
	}
	f2.Close()

	roomData := &editor.Room{
		Width:  64,
		Height: 64,
		TilesetLayers: []editor.TilesetLayer{
			{
				Name:        "Tiles 1",
				TilesetPath: tilesetPath,
				TileWidth:   16,
				TileHeight:  16,
				Tiles: []editor.Tile{
					{Index: "0"},
				},
			},
		},
		ObjectLayers: []editor.ObjectLayer{
			{
				Objects: []editor.Object{
					{
						Type:                 "Hero",
						SpriteSheet:          spriteSheetPath,
						SubSpriteWidth:       16,
						SubSpriteHeight:      16,
						SubSpritesTotalCount: 4,
						Coordinates:          editor.Coordinates{X: 10, Y: 10},
					},
				},
			},
		},
	}

	roomFile := tempDir + "/tileset_room.json"
	if err := editor.SaveRoomToFile(roomData, roomFile); err != nil {
		t.Fatalf("Failed to save room file: %v", err)
	}

	if err := Add("tileset_room", roomFile); err != nil {
		t.Fatalf("Failed to add room: %v", err)
	}
	if err := SetCurrentRoom("tileset_room"); err != nil {
		t.Fatalf("Failed to set current room: %v", err)
	}

	screen := ebiten.NewImage(64, 64)

	// First Painter call populates caches
	Painter(screen)

	// Remove source image files from disk
	if err := os.Remove(tilesetPath); err != nil {
		t.Fatalf("Failed to remove tileset file: %v", err)
	}
	if err := os.Remove(spriteSheetPath); err != nil {
		t.Fatalf("Failed to remove sprite sheet file: %v", err)
	}

	// Second Painter call should succeed without error/panic using cached images
	Painter(screen)
}
