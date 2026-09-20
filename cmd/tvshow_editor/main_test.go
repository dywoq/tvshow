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

	drawObjectLetter(img, "E", 10, 10, 20, 20, clr)

	c := img.At(12, 12).(color.RGBA)
	if c.A == 0 {
		t.Errorf("expected letter drawn at (10,10), got transparent pixel at (12,12)")
	}
}

func TestTilePaletteToolsAndUndoRedo(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	tempDir := t.TempDir()
	tsPath := filepath.Join(tempDir, "tileset.png")

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

	app.room = &editor.Room{
		Width:  640,
		Height: 480,
		ObjectLayers: []editor.ObjectLayer{
			{
				Name: "ObjLayer",
				Objects: []editor.Object{
					{Type: "Monster", Coordinates: editor.Coordinates{X: 100, Y: 100}},
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

	// 1. Verify palette extraction
	if len(app.paletteButtons) != 4 {
		t.Errorf("expected 4 palette buttons, got %d", len(app.paletteButtons))
	}

	// Select tile #1
	if btn, ok := app.paletteButtons["1"]; ok {
		btn.OnTapped()
	}
	if app.selectedPaletteTileIdx != "1" {
		t.Errorf("expected selectedPaletteTileIdx '1', got %q", app.selectedPaletteTileIdx)
	}

	// 2. Place tile & Test Undo/Redo
	app.placeTileAtRoomCoords(32, 32)
	if len(app.room.TilesetLayers[0].Tiles) != 1 {
		t.Fatalf("expected 1 tile after placement, got %d", len(app.room.TilesetLayers[0].Tiles))
	}

	app.undo()
	if len(app.room.TilesetLayers[0].Tiles) != 0 {
		t.Errorf("expected 0 tiles after undo, got %d", len(app.room.TilesetLayers[0].Tiles))
	}

	app.redo()
	if len(app.room.TilesetLayers[0].Tiles) != 1 {
		t.Errorf("expected 1 tile after redo, got %d", len(app.room.TilesetLayers[0].Tiles))
	}

	// 3. Test Selection Box creation & dragging selected items
	app.toolSelect.SetSelected("Selection Tool")
	if app.activeTool != ToolSelection {
		t.Errorf("expected activeTool ToolSelection, got %v", app.activeTool)
	}

	containerSize := fyne.NewSize(640, 480)
	// Create selection box covering (0,0) to (120, 120) which contains tile (32, 32) and object (100, 100)
	app.handlePreviewDrag(fyne.NewPos(0, 0), containerSize)
	app.handlePreviewDrag(fyne.NewPos(120, 120), containerSize)
	app.handlePreviewDragEnd()

	if !app.hasSelectionBox {
		t.Errorf("expected hasSelectionBox to be true")
	}
	if len(app.selectedTiles) != 1 {
		t.Errorf("expected 1 selected tile in box, got %d", len(app.selectedTiles))
	}
	if len(app.selectedObjects) != 1 {
		t.Errorf("expected 1 selected object in box, got %d", len(app.selectedObjects))
	}

	// Move selection box by dragging from (50, 50) to (70, 70) (delta +20, +20)
	app.handlePreviewDrag(fyne.NewPos(50, 50), containerSize)
	app.handlePreviewDrag(fyne.NewPos(70, 70), containerSize)
	app.handlePreviewDragEnd()

	obj := app.room.ObjectLayers[0].Objects[0]
	if obj.Coordinates.X != 120 || obj.Coordinates.Y != 120 {
		t.Errorf("expected selected object moved to (120, 120), got (%d, %d)", obj.Coordinates.X, obj.Coordinates.Y)
	}

	tile := app.room.TilesetLayers[0].Tiles[0]
	// Tile is snapped to 16x16 grid: (32+20=52) -> (52/16)*16 = 48
	if tile.Coordinates.X != 48 || tile.Coordinates.Y != 48 {
		t.Errorf("expected selected tile moved and snapped to (48, 48), got (%d, %d)", tile.Coordinates.X, tile.Coordinates.Y)
	}

	// Delete selected items using deleteSelectedItems
	app.deleteSelectedItems()
	if len(app.room.ObjectLayers[0].Objects) != 0 {
		t.Errorf("expected 0 objects after deletion, got %d", len(app.room.ObjectLayers[0].Objects))
	}
	if len(app.room.TilesetLayers[0].Tiles) != 0 {
		t.Errorf("expected 0 tiles after deletion, got %d", len(app.room.TilesetLayers[0].Tiles))
	}

	// Test Brush tool continuous drag
	app.toolSelect.SetSelected("Brush Tool")
	app.handlePreviewDrag(fyne.NewPos(64, 64), containerSize)
	app.handlePreviewDrag(fyne.NewPos(80, 64), containerSize)
	app.handlePreviewDragEnd()

	if len(app.room.TilesetLayers[0].Tiles) < 2 {
		t.Errorf("expected at least 2 tiles placed via brush drag, got %d", len(app.room.TilesetLayers[0].Tiles))
	}

	// 6. Test Secondary Tap (right click menu)
	app.handlePreviewSecondaryTap(fyne.NewPos(50, 50), containerSize)
}
