package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"tvshow/game/editor"
)

func TestDrawObjectLetter(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	clr := color.RGBA{R: 255, G: 0, B: 0, A: 255}

	drawObjectLetter(img, "E", 10, 10, clr)

	// Check that pixels inside the draw box (10, 10) are no longer completely transparent black
	c := img.At(12, 12).(color.RGBA)
	if c.A == 0 {
		t.Errorf("expected letter drawn at (10,10), got transparent pixel at (12,12)")
	}
}

func TestTilePaletteAndPlacement(t *testing.T) {
	// Initialize test app
	a := test.NewApp()
	defer a.Quit()

	tempDir := t.TempDir()
	tsPath := filepath.Join(tempDir, "tileset.png")

	// Create a 32x32 tileset image (4 tiles of 16x16)
	tsImg := image.NewRGBA(image.Rect(0, 0, 32, 32))
	f, err := os.Create(tsPath)
	if err != nil {
		t.Fatalf("failed to create temp tileset image: %v", err)
	}
	if err := png.Encode(f, tsImg); err != nil {
		f.Close()
		t.Fatalf("failed to encode temp tileset image: %v", err)
	}
	f.Close()

	app := newEditorAppWithApp(a)
	app.baseDir = tempDir

	// Setup room with 1 tileset layer
	app.room = &editor.Room{
		Width:  640,
		Height: 480,
		ObjectLayers: []editor.ObjectLayer{
			{
				Name: "ObjLayer",
				Objects: []editor.Object{
					{Type: "Monster", Coordinates: editor.Coordinates{X: 32, Y: 32}},
				},
			},
		},
		TilesetLayers: []editor.TilesetLayer{
			{
				Name:        "TSLayer",
				TilesetPath: "tileset.png",
				TileWidth:   16,
				TileHeight:  16,
				Tiles:       []editor.Tile{},
			},
		},
	}

	app.buildUI()

	// Verify palette rebuilt with 4 tiles (#0, #1, #2, #3)
	if len(app.paletteButtons) != 4 {
		t.Errorf("expected 4 palette buttons, got %d", len(app.paletteButtons))
	}

	// Select palette tile #2
	if btn, ok := app.paletteButtons["2"]; ok {
		btn.OnTapped()
	} else {
		t.Fatalf("palette tile #2 button not found")
	}

	if app.selectedPaletteTileIdx != "2" {
		t.Errorf("expected selectedPaletteTileIdx to be '2', got %q", app.selectedPaletteTileIdx)
	}

	// Place tile at room coordinates (25, 25) -> should snap to grid (16, 16)
	app.placeTileAtRoomCoords(25, 25)

	layer := app.room.TilesetLayers[0]
	if len(layer.Tiles) != 1 {
		t.Fatalf("expected 1 tile placed in tileset layer, got %d", len(layer.Tiles))
	}

	tile := layer.Tiles[0]
	if tile.Index != "2" {
		t.Errorf("expected tile index '2', got %q", tile.Index)
	}
	if tile.Coordinates == nil || tile.Coordinates.X != 16 || tile.Coordinates.Y != 16 {
		t.Errorf("expected tile coordinates (16, 16), got %+v", tile.Coordinates)
	}

	// Place palette tile #3 at same grid coordinates (18, 18) -> should replace tile at (16, 16)
	if btn, ok := app.paletteButtons["3"]; ok {
		btn.OnTapped()
	}
	app.placeTileAtRoomCoords(18, 18)

	layer = app.room.TilesetLayers[0]
	if len(layer.Tiles) != 1 {
		t.Fatalf("expected 1 tile after replacement, got %d", len(layer.Tiles))
	}
	if layer.Tiles[0].Index != "3" {
		t.Errorf("expected tile index '3' after replacement, got %q", layer.Tiles[0].Index)
	}

	// Test handlePreviewTap coordinate conversion
	containerSize := fyne.NewSize(640, 480)
	tapPos := fyne.NewPos(320, 240) // center -> (320, 240)
	app.handlePreviewTap(tapPos, containerSize)

	layer = app.room.TilesetLayers[0]
	if len(layer.Tiles) != 2 {
		t.Fatalf("expected 2 tiles after tap placement, got %d", len(layer.Tiles))
	}
	t2 := layer.Tiles[1]
	if t2.Coordinates == nil || t2.Coordinates.X != 320 || t2.Coordinates.Y != 240 {
		t.Errorf("expected tile at (320, 240), got %+v", t2.Coordinates)
	}
}
