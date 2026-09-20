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

	// 3. Test Tools (Selection tool)
	app.toolSelect.SetSelected("Selection Tool")
	if app.activeTool != ToolSelection {
		t.Errorf("expected activeTool ToolSelection, got %v", app.activeTool)
	}

	containerSize := fyne.NewSize(640, 480)
	// Tap on tile at (32, 32)
	app.handlePreviewTap(fyne.NewPos(32, 32), containerSize)
	if app.selectedTileIdx != 0 {
		t.Errorf("expected selectedTileIdx 0, got %d", app.selectedTileIdx)
	}

	// 4. Test Brush tool continuous drag
	app.toolSelect.SetSelected("Brush Tool")
	app.handlePreviewDrag(fyne.NewPos(64, 64), containerSize)
	app.handlePreviewDrag(fyne.NewPos(80, 64), containerSize)
	app.handlePreviewDragEnd()

	if len(app.room.TilesetLayers[0].Tiles) < 3 {
		t.Errorf("expected at least 3 tiles placed via brush drag, got %d", len(app.room.TilesetLayers[0].Tiles))
	}

	// 5. Test Object Dragging in room preview
	// Drag object at (100, 100) to (150, 150)
	app.handlePreviewDrag(fyne.NewPos(100, 100), containerSize)
	app.handlePreviewDrag(fyne.NewPos(150, 150), containerSize)
	app.handlePreviewDragEnd()

	obj := app.room.ObjectLayers[0].Objects[0]
	if obj.Coordinates.X != 150 || obj.Coordinates.Y != 150 {
		t.Errorf("expected object moved to (150, 150), got (%d, %d)", obj.Coordinates.X, obj.Coordinates.Y)
	}

	// Undo object move
	app.undo()
	objUndo := app.room.ObjectLayers[0].Objects[0]
	if objUndo.Coordinates.X != 100 || objUndo.Coordinates.Y != 100 {
		t.Errorf("expected object position reverted to (100, 100) after undo, got (%d, %d)", objUndo.Coordinates.X, objUndo.Coordinates.Y)
	}
}
