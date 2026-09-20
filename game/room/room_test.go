package room

import (
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
