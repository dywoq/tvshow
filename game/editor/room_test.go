package editor

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveRoom(t *testing.T) {
	jsonStr := `{
		"width": 320,
		"height": 240,
		"object_layers": [
			{
				"objects": [
					{
						"type": "player",
						"attributes": ["hero", "spawn"],
						"coordinates": {"x": 10, "y": 20}
					}
				]
			}
		],
		"tileset_layers": [
			{
				"tileset_path": "tiles.png",
				"attributes": ["background"],
				"tile_width": 16,
				"tile_height": 16,
				"tiles": [
					{"index": "0", "coordinates": {"x": 0, "y": 0}},
					{"index": "1"}
				]
			}
		]
	}`

	room, err := LoadRoomFromBytes([]byte(jsonStr))
	if err != nil {
		t.Fatalf("LoadRoomFromBytes failed: %v", err)
	}

	if room.Width != 320 || room.Height != 240 {
		t.Errorf("Expected 320x240, got %dx%d", room.Width, room.Height)
	}
	if len(room.ObjectLayers) != 1 {
		t.Fatalf("Expected 1 object layer, got %d", len(room.ObjectLayers))
	}
	if len(room.ObjectLayers[0].Objects) != 1 {
		t.Fatalf("Expected 1 object, got %d", len(room.ObjectLayers[0].Objects))
	}
	obj := room.ObjectLayers[0].Objects[0]
	if obj.Type != "player" || obj.Coordinates.X != 10 || obj.Coordinates.Y != 20 {
		t.Errorf("Unexpected object properties: %+v", obj)
	}

	if len(room.TilesetLayers) != 1 {
		t.Fatalf("Expected 1 tileset layer, got %d", len(room.TilesetLayers))
	}
	tsLayer := room.TilesetLayers[0]
	if tsLayer.TilesetPath != "tiles.png" || tsLayer.TileWidth != 16 || tsLayer.TileHeight != 16 {
		t.Errorf("Unexpected tileset layer properties: %+v", tsLayer)
	}

	// Test Save & Load cycle
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_room.json")

	if err := SaveRoomToFile(room, filePath); err != nil {
		t.Fatalf("SaveRoomToFile failed: %v", err)
	}

	loadedRoom, err := LoadRoomFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadRoomFromFile failed: %v", err)
	}

	if loadedRoom.Width != room.Width || loadedRoom.Height != room.Height {
		t.Errorf("Loaded room dimensions mismatch")
	}
}

func TestRenderTilesetLayer(t *testing.T) {
	tempDir := t.TempDir()
	tilesetPath := filepath.Join(tempDir, "tileset.png")

	// Create a 32x16 dummy tileset image (2 tiles of 16x16: Tile 0 red, Tile 1 green)
	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, red)
		}
		for x := 16; x < 32; x++ {
			img.Set(x, y, green)
		}
	}

	f, err := os.Create(tilesetPath)
	if err != nil {
		t.Fatalf("Failed to create temp tileset file: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Failed to encode tileset PNG: %v", err)
	}
	f.Close()

	room := &Room{
		Width:  64,
		Height: 64,
		TilesetLayers: []TilesetLayer{
			{
				TilesetPath: "tileset.png",
				TileWidth:   16,
				TileHeight:  16,
				Tiles: []Tile{
					{Index: "0", Coordinates: &Coordinates{X: 0, Y: 0}},
					{Index: "1", Coordinates: &Coordinates{X: 16, Y: 0}},
				},
			},
		},
	}

	renderedImg, err := RenderTilesetLayer(&room.TilesetLayers[0], room.Width, room.Height, tempDir)
	if err != nil {
		t.Fatalf("RenderTilesetLayer failed: %v", err)
	}

	// Check pixel colors at (0,0) [Red] and (16,0) [Green]
	r0, g0, b0, a0 := renderedImg.At(0, 0).RGBA()
	if r0>>8 != 255 || g0>>8 != 0 || b0>>8 != 0 || a0>>8 != 255 {
		t.Errorf("Expected red at (0,0), got RGBA(%d,%d,%d,%d)", r0>>8, g0>>8, b0>>8, a0>>8)
	}

	r1, g1, b1, a1 := renderedImg.At(16, 0).RGBA()
	if r1>>8 != 0 || g1>>8 != 255 || b1>>8 != 0 || a1>>8 != 255 {
		t.Errorf("Expected green at (16,0), got RGBA(%d,%d,%d,%d)", r1>>8, g1>>8, b1>>8, a1>>8)
	}

	// Test RenderRoomComposite
	compositeImg, err := RenderRoomComposite(room, tempDir)
	if err != nil {
		t.Fatalf("RenderRoomComposite failed: %v", err)
	}

	cr0, cg0, cb0, _ := compositeImg.At(0, 0).RGBA()
	if cr0>>8 != 255 || cg0>>8 != 0 || cb0>>8 != 0 {
		t.Errorf("Composite: Expected red at (0,0)")
	}
}

func TestRenderTilesetLayerMissingFile(t *testing.T) {
	layer := &TilesetLayer{
		TilesetPath: "non_existent_file.png",
		TileWidth:   16,
		TileHeight:  16,
	}

	_, err := RenderTilesetLayer(layer, 100, 100, os.TempDir())
	if err == nil {
		t.Errorf("Expected error for non-existent tileset path, got nil")
	}
}

func TestObjectWidthAndHeight(t *testing.T) {
	jsonStr := `{
		"width": 100,
		"height": 100,
		"object_layers": [
			{
				"objects": [
					{
						"type": "box",
						"width": 32,
						"height": 32,
						"coordinates": {"x": 5, "y": 5}
					}
				]
			}
		]
	}`

	room, err := LoadRoomFromBytes([]byte(jsonStr))
	if err != nil {
		t.Fatalf("LoadRoomFromBytes failed: %v", err)
	}

	obj := room.ObjectLayers[0].Objects[0]
	if obj.Width != 32 || obj.Height != 32 {
		t.Errorf("Expected object width and height 32x32, got %dx%d", obj.Width, obj.Height)
	}
}
