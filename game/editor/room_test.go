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

func TestObjectSpriteSheet(t *testing.T) {
	tempDir := t.TempDir()
	spriteSheetPath := filepath.Join(tempDir, "hero_sheet.png")

	// Create a 32x32 sprite sheet image (4 sub-sprites of 16x16: 0=red, 1=green, 2=blue, 3=white)
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, red)
		}
		for x := 16; x < 32; x++ {
			img.Set(x, y, green)
		}
	}
	for y := 16; y < 32; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, blue)
		}
		for x := 16; x < 32; x++ {
			img.Set(x, y, white)
		}
	}

	f, err := os.Create(spriteSheetPath)
	if err != nil {
		t.Fatalf("Failed to create temp sprite sheet: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Failed to encode sprite sheet PNG: %v", err)
	}
	f.Close()

	obj := Object{
		Type:            "hero",
		SpriteSheet:     "hero_sheet.png",
		SubSpriteWidth:  16,
		SubSpriteHeight: 16,
		Coordinates:     Coordinates{X: 10, Y: 10},
	}

	err = ProcessObjectSpriteSheet(&obj, tempDir)
	if err != nil {
		t.Fatalf("ProcessObjectSpriteSheet failed: %v", err)
	}

	if obj.SubSpritesTotalCount != 4 {
		t.Errorf("Expected sub_sprites_total_count 4, got %d", obj.SubSpritesTotalCount)
	}
	if len(obj.Sprites) != 4 {
		t.Fatalf("Expected 4 sprites in array, got %d", len(obj.Sprites))
	}
	for i, s := range obj.Sprites {
		if s.Index != i {
			t.Errorf("Expected sprite index %d, got %d", i, s.Index)
		}
	}

	// Test LoadSubSprite
	sub0, err := LoadSubSprite(&obj, 0, tempDir)
	if err != nil {
		t.Fatalf("LoadSubSprite 0 failed: %v", err)
	}
	r, g, b, _ := sub0.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("Expected red for sub-sprite 0")
	}

	sub3, err := LoadSubSprite(&obj, 3, tempDir)
	if err != nil {
		t.Fatalf("LoadSubSprite 3 failed: %v", err)
	}
	r3, g3, b3, _ := sub3.At(0, 0).RGBA()
	if r3>>8 != 255 || g3>>8 != 255 || b3>>8 != 255 {
		t.Errorf("Expected white for sub-sprite 3")
	}

	// Test LoadSubSprites
	allSub, err := LoadSubSprites(&obj, tempDir)
	if err != nil {
		t.Fatalf("LoadSubSprites failed: %v", err)
	}
	if len(allSub) != 4 {
		t.Errorf("Expected 4 sub-sprites, got %d", len(allSub))
	}

	// Test JSON roundtrip with sprite sheet fields
	room := &Room{
		Width:  100,
		Height: 100,
		ObjectLayers: []ObjectLayer{
			{
				Objects: []Object{obj},
			},
		},
	}

	data, err := SaveRoomToBytes(room)
	if err != nil {
		t.Fatalf("SaveRoomToBytes failed: %v", err)
	}

	loadedRoom, err := LoadRoomFromBytes(data)
	if err != nil {
		t.Fatalf("LoadRoomFromBytes failed: %v", err)
	}

	loadedObj := loadedRoom.ObjectLayers[0].Objects[0]
	if loadedObj.SpriteSheet != "hero_sheet.png" {
		t.Errorf("Expected sprite_sheet 'hero_sheet.png', got %q", loadedObj.SpriteSheet)
	}
	if loadedObj.SubSpriteWidth != 16 || loadedObj.SubSpriteHeight != 16 {
		t.Errorf("Expected sub-sprite 16x16, got %dx%d", loadedObj.SubSpriteWidth, loadedObj.SubSpriteHeight)
	}
	if loadedObj.SubSpritesTotalCount != 4 {
		t.Errorf("Expected sub_sprites_total_count 4, got %d", loadedObj.SubSpritesTotalCount)
	}
	if len(loadedObj.Sprites) != 4 {
		t.Fatalf("Expected 4 sprites in array, got %d", len(loadedObj.Sprites))
	}
	if loadedObj.Sprites[2].Index != 2 {
		t.Errorf("Expected sprite index 2, got %d", loadedObj.Sprites[2].Index)
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

func TestEditObjectAndTile(t *testing.T) {
	objLayer := ObjectLayer{
		Objects: []Object{
			{Type: "chest", Width: 16, Height: 16, Coordinates: Coordinates{X: 10, Y: 10}},
		},
	}

	err := objLayer.EditObject(0, Object{
		Type:        "chest_opened",
		Width:       20,
		Height:      20,
		Coordinates: Coordinates{X: 15, Y: 15},
		Attributes:  []string{"open", "looted"},
	})
	if err != nil {
		t.Fatalf("EditObject failed: %v", err)
	}
	if objLayer.Objects[0].Type != "chest_opened" || objLayer.Objects[0].Width != 20 {
		t.Errorf("Object not updated correctly: %+v", objLayer.Objects[0])
	}

	if err := objLayer.EditObject(5, Object{}); err == nil {
		t.Errorf("Expected error for out-of-bounds EditObject, got nil")
	}

	tsLayer := TilesetLayer{
		Tiles: []Tile{
			{Index: "0", Coordinates: &Coordinates{X: 0, Y: 0}},
		},
	}

	err = tsLayer.EditTile(0, Tile{
		Index:       "5",
		Coordinates: &Coordinates{X: 16, Y: 16},
	})
	if err != nil {
		t.Fatalf("EditTile failed: %v", err)
	}
	if tsLayer.Tiles[0].Index != "5" || tsLayer.Tiles[0].Coordinates.X != 16 {
		t.Errorf("Tile not updated correctly: %+v", tsLayer.Tiles[0])
	}

	if err := tsLayer.EditTile(5, Tile{}); err == nil {
		t.Errorf("Expected error for out-of-bounds EditTile, got nil")
	}
}
