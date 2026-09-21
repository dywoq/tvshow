package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"tvshow/game/editor"
)

type EditorTool string

const (
	ToolPlace     EditorTool = "place"
	ToolBrush     EditorTool = "brush"
	ToolSelection EditorTool = "selection"
)

type SelectedTileRef struct {
	LayerIdx int
	TileIdx  int
}

type SelectedObjRef struct {
	LayerIdx int
	ObjIdx   int
}

type EditorApp struct {
	fyneApp     fyne.App
	window      fyne.Window
	room        *editor.Room
	currentPath string
	baseDir     string

	// UI Components
	previewContainer *fyne.Container
	statusLabel      *widget.Label
	roomWidthEntry   *widget.Entry
	roomHeightEntry  *widget.Entry

	// Object Layer controls
	objLayerSelect *widget.Select
	objList        *widget.List
	selectedObjIdx widget.ListItemID

	// Tileset Layer controls
	tsLayerSelect          *widget.Select
	tsPathEntry            *widget.Entry
	tsAttrEntry            *widget.Entry
	tileWEntry             *widget.Entry
	tileHEntry             *widget.Entry
	tileList               *widget.List
	selectedTileIdx        widget.ListItemID
	paletteContainer       *fyne.Container
	paletteButtons         map[string]*widget.Button
	selectedPaletteTileIdx string
	tilesetCache           map[string]image.Image

	// Tool controls
	activeTool EditorTool
	toolSelect *widget.RadioGroup
	lastBrushX int
	lastBrushY int
	isDragging bool

	// Object dragging state
	draggedObjLayerIdx int
	draggedObjIdx      int

	// Selection state
	isSelecting       bool
	selectionStart    editor.Coordinates
	selectionEnd      editor.Coordinates
	hasSelectionBox   bool
	selectedTiles     []SelectedTileRef
	selectedObjects   []SelectedObjRef
	isMovingSelection bool
	moveStartCoords   editor.Coordinates

	// Undo / Redo history
	undoStack []*editor.Room
	redoStack []*editor.Room
}

func cloneRoom(r *editor.Room) *editor.Room {
	if r == nil {
		return nil
	}
	data, err := editor.SaveRoomToBytes(r)
	if err != nil {
		return r
	}
	cloned, err := editor.LoadRoomFromBytes(data)
	if err != nil {
		return r
	}
	return cloned
}

func (e *EditorApp) recordUndo() {
	snapshot := cloneRoom(e.room)
	e.undoStack = append(e.undoStack, snapshot)
	e.redoStack = nil
}

func (e *EditorApp) undo() {
	if len(e.undoStack) == 0 {
		e.statusLabel.SetText("Nothing to undo")
		return
	}
	e.redoStack = append(e.redoStack, cloneRoom(e.room))
	lastIdx := len(e.undoStack) - 1
	e.room = e.undoStack[lastIdx]
	e.undoStack = e.undoStack[:lastIdx]

	e.refreshUI()
	e.statusLabel.SetText("Undo performed")
}

func (e *EditorApp) redo() {
	if len(e.redoStack) == 0 {
		e.statusLabel.SetText("Nothing to redo")
		return
	}
	e.undoStack = append(e.undoStack, cloneRoom(e.room))
	lastIdx := len(e.redoStack) - 1
	e.room = e.redoStack[lastIdx]
	e.redoStack = e.redoStack[:lastIdx]

	e.refreshUI()
	e.statusLabel.SetText("Redo performed")
}

func newEditorApp() *EditorApp {
	return newEditorAppWithApp(app.New())
}

func newEditorAppWithApp(a fyne.App) *EditorApp {
	w := a.NewWindow("Room Editor - TV Show")
	w.Resize(fyne.NewSize(1024, 700))

	pwd, _ := os.Getwd()

	e := &EditorApp{
		fyneApp:         a,
		window:          w,
		baseDir:         pwd,
		selectedObjIdx:     -1,
		selectedTileIdx:    -1,
		activeTool:         ToolBrush,
		lastBrushX:         -1,
		lastBrushY:         -1,
		draggedObjLayerIdx: -1,
		draggedObjIdx:      -1,
		room: &editor.Room{
			Width:  640,
			Height: 480,
			ObjectLayers: []editor.ObjectLayer{
				{
					Name:       "Default Object Layer",
					Attributes: []string{},
					Objects:    []editor.Object{},
				},
			},
			TilesetLayers: []editor.TilesetLayer{
				{
					Name:        "Default Tileset Layer",
					TilesetPath: "",
					Attributes:  []string{},
					TileWidth:   16,
					TileHeight:  16,
					Tiles:       []editor.Tile{},
				},
			},
		},
	}

	return e
}

func (e *EditorApp) buildUI() fyne.CanvasObject {
	e.statusLabel = widget.NewLabel("Ready")

	// Room Settings
	e.roomWidthEntry = widget.NewEntry()
	e.roomWidthEntry.SetText(strconv.Itoa(e.room.Width))
	e.roomHeightEntry = widget.NewEntry()
	e.roomHeightEntry.SetText(strconv.Itoa(e.room.Height))

	roomForm := widget.NewForm(
		widget.NewFormItem("Width", e.roomWidthEntry),
		widget.NewFormItem("Height", e.roomHeightEntry),
	)
	updateRoomBtn := widget.NewButton("Apply Room Dimensions", func() {
		w, errW := strconv.Atoi(e.roomWidthEntry.Text)
		h, errH := strconv.Atoi(e.roomHeightEntry.Text)
		if errW != nil || errH != nil || w <= 0 || h <= 0 {
			dialog.ShowError(fmt.Errorf("invalid width or height"), e.window)
			return
		}
		e.room.Width = w
		e.room.Height = h
		e.refreshPreview()
	})
	roomSettingsCard := widget.NewCard("Room Dimensions", "", container.NewVBox(roomForm, updateRoomBtn))

	// Object Layers Tab
	objTab := e.buildObjectLayersTab()

	// Tileset Layers Tab
	tsTab := e.buildTilesetLayersTab()

	tabs := container.NewAppTabs(
		container.NewTabItem("Room", roomSettingsCard),
		container.NewTabItem("Object Group", objTab),
		container.NewTabItem("Tile Set Group", tsTab),
	)

	// Visual Preview panel
	e.previewContainer = container.NewStack()

	refreshPreviewBtn := widget.NewButton("Refresh Preview", func() {
		e.refreshPreview()
	})

	rightPanel := container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("Room Preview", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), refreshPreviewBtn),
		nil, nil, nil,
		e.previewContainer,
	)

	split := container.NewHSplit(tabs, rightPanel)
	split.Offset = 0.5

	// Toolbar
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentIcon(), func() { e.newRoom() }),
		widget.NewToolbarAction(theme.FolderOpenIcon(), func() { e.openRoom() }),
		widget.NewToolbarAction(theme.DocumentSaveIcon(), func() { e.saveRoom() }),
		widget.NewToolbarAction(theme.DownloadIcon(), func() { e.exportRoom() }),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.HistoryIcon(), func() { e.undo() }),
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() { e.redo() }),
	)

	// Register Undo (Ctrl+Z), Redo (Ctrl+Shift+Z), and Delete shortcuts
	undoShortcutCtrl := &desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl}
	undoShortcutCmd := &desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierSuper}
	redoShortcutCtrl := &desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}
	redoShortcutCmd := &desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierSuper | fyne.KeyModifierShift}
	deleteShortcut := &desktop.CustomShortcut{KeyName: fyne.KeyDelete}
	backspaceShortcut := &desktop.CustomShortcut{KeyName: fyne.KeyBackspace}

	e.window.Canvas().AddShortcut(undoShortcutCtrl, func(shortcut fyne.Shortcut) { e.undo() })
	e.window.Canvas().AddShortcut(undoShortcutCmd, func(shortcut fyne.Shortcut) { e.undo() })
	e.window.Canvas().AddShortcut(redoShortcutCtrl, func(shortcut fyne.Shortcut) { e.redo() })
	e.window.Canvas().AddShortcut(redoShortcutCmd, func(shortcut fyne.Shortcut) { e.redo() })
	e.window.Canvas().AddShortcut(deleteShortcut, func(shortcut fyne.Shortcut) { e.deleteSelectedItems() })
	e.window.Canvas().AddShortcut(backspaceShortcut, func(shortcut fyne.Shortcut) { e.deleteSelectedItems() })

	mainContent := container.NewBorder(
		toolbar,
		e.statusLabel,
		nil, nil,
		split,
	)

	e.refreshUI()
	return mainContent
}

func (e *EditorApp) buildObjectLayersTab() fyne.CanvasObject {
	e.objLayerSelect = widget.NewSelect([]string{}, func(selected string) {
		e.selectedObjIdx = -1
		e.refreshObjectList()
	})

	addLayerBtn := widget.NewButton("Add Object Layer", func() {
		e.room.ObjectLayers = append(e.room.ObjectLayers, editor.ObjectLayer{
			Name:       fmt.Sprintf("Object Layer %d", len(e.room.ObjectLayers)+1),
			Attributes: []string{},
			Objects:    []editor.Object{},
		})
		e.refreshObjectLayerSelect()
	})

	removeLayerBtn := widget.NewButton("Delete Layer", func() {
		idx := e.objLayerSelect.SelectedIndex()
		if idx < 0 || idx >= len(e.room.ObjectLayers) {
			return
		}
		e.room.ObjectLayers = append(e.room.ObjectLayers[:idx], e.room.ObjectLayers[idx+1:]...)
		e.refreshObjectLayerSelect()
	})

	layerHeader := container.NewVBox(
		container.NewHBox(widget.NewLabel("Layer:"), e.objLayerSelect, addLayerBtn, removeLayerBtn),
	)

	e.objList = widget.NewList(
		func() int {
			idx := e.objLayerSelect.SelectedIndex()
			if idx < 0 || idx >= len(e.room.ObjectLayers) {
				return 0
			}
			return len(e.room.ObjectLayers[idx].Objects)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Object Info")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			idx := e.objLayerSelect.SelectedIndex()
			if idx >= 0 && idx < len(e.room.ObjectLayers) {
				obj := e.room.ObjectLayers[idx].Objects[id]
				ssInfo := ""
				if obj.SpriteSheet != "" {
					ssInfo = fmt.Sprintf(" | SpriteSheet: %s (%dx%d, Total: %d)", obj.SpriteSheet, obj.SubSpriteWidth, obj.SubSpriteHeight, obj.SubSpritesTotalCount)
				}
				item.(*widget.Label).SetText(fmt.Sprintf("[%d] Type: %s | Size: %dx%d | Pos: (%d, %d) | Attrs: %v%s",
					id, obj.Type, obj.Width, obj.Height, obj.Coordinates.X, obj.Coordinates.Y, obj.Attributes, ssInfo))
			}
		},
	)
	e.objList.OnSelected = func(id widget.ListItemID) {
		e.selectedObjIdx = id
	}

	addObjBtn := widget.NewButton("Add Object", func() {
		layerIdx := e.objLayerSelect.SelectedIndex()
		if layerIdx < 0 || layerIdx >= len(e.room.ObjectLayers) {
			dialog.ShowError(fmt.Errorf("select an object layer first"), e.window)
			return
		}

		typeEntry := widget.NewEntry()
		wEntry := widget.NewEntry()
		wEntry.SetText("16")
		hEntry := widget.NewEntry()
		hEntry.SetText("16")
		attrsEntry := widget.NewEntry()
		xEntry := widget.NewEntry()
		xEntry.SetText("0")
		yEntry := widget.NewEntry()
		yEntry.SetText("0")
		spriteSheetEntry := widget.NewEntry()
		subWEntry := widget.NewEntry()
		subHEntry := widget.NewEntry()

		form := widget.NewForm(
			widget.NewFormItem("Type", typeEntry),
			widget.NewFormItem("Width", wEntry),
			widget.NewFormItem("Height", hEntry),
			widget.NewFormItem("Attributes (comma-separated)", attrsEntry),
			widget.NewFormItem("X", xEntry),
			widget.NewFormItem("Y", yEntry),
			widget.NewFormItem("Sprite Sheet Path (optional)", spriteSheetEntry),
			widget.NewFormItem("Sub-Sprite Width (optional)", subWEntry),
			widget.NewFormItem("Sub-Sprite Height (optional)", subHEntry),
		)

		dialog.ShowCustomConfirm("Add Object", "Add", "Cancel", form, func(ok bool) {
			if !ok {
				return
			}
			x, _ := strconv.Atoi(xEntry.Text)
			y, _ := strconv.Atoi(yEntry.Text)
			w, _ := strconv.Atoi(wEntry.Text)
			h, _ := strconv.Atoi(hEntry.Text)
			subW, _ := strconv.Atoi(subWEntry.Text)
			subH, _ := strconv.Atoi(subHEntry.Text)
			var attrs []string
			if attrsEntry.Text != "" {
				for _, a := range strings.Split(attrsEntry.Text, ",") {
					attrs = append(attrs, strings.TrimSpace(a))
				}
			}

			newObj := editor.Object{
				Type:            typeEntry.Text,
				Width:           w,
				Height:          h,
				Attributes:      attrs,
				Coordinates:     editor.Coordinates{X: x, Y: y},
				SpriteSheet:     spriteSheetEntry.Text,
				SubSpriteWidth:  subW,
				SubSpriteHeight: subH,
			}

			if newObj.SpriteSheet != "" {
				if err := editor.ProcessObjectSpriteSheet(&newObj, e.baseDir); err != nil {
					dialog.ShowError(fmt.Errorf("failed to process sprite sheet: %w", err), e.window)
				}
			}

			e.recordUndo()
			e.room.ObjectLayers[layerIdx].Objects = append(e.room.ObjectLayers[layerIdx].Objects, newObj)
			e.refreshObjectList()
			e.refreshPreview()
		}, e.window)
	})

	editObjBtn := widget.NewButton("Edit Selected Object", func() {
		e.editSelectedObjectDialog()
	})

	deleteObjBtn := widget.NewButton("Delete Selected Object", func() {
		layerIdx := e.objLayerSelect.SelectedIndex()
		if layerIdx < 0 || layerIdx >= len(e.room.ObjectLayers) {
			return
		}
		objs := e.room.ObjectLayers[layerIdx].Objects
		if len(objs) == 0 {
			return
		}
		targetIdx := e.selectedObjIdx
		if targetIdx < 0 || targetIdx >= len(objs) {
			targetIdx = len(objs) - 1
		}
		e.recordUndo()
		e.room.ObjectLayers[layerIdx].Objects = append(objs[:targetIdx], objs[targetIdx+1:]...)
		e.selectedObjIdx = -1
		e.refreshObjectList()
		e.refreshPreview()
	})

	objControls := container.NewHBox(addObjBtn, editObjBtn, deleteObjBtn)

	return container.NewBorder(layerHeader, objControls, nil, nil, e.objList)
}

func (e *EditorApp) editSelectedObjectDialog() {
	layerIdx := e.objLayerSelect.SelectedIndex()
	if layerIdx < 0 || layerIdx >= len(e.room.ObjectLayers) {
		dialog.ShowError(fmt.Errorf("select an object layer first"), e.window)
		return
	}
	objs := e.room.ObjectLayers[layerIdx].Objects
	if len(objs) == 0 {
		dialog.ShowError(fmt.Errorf("no objects in selected layer"), e.window)
		return
	}
	targetIdx := e.selectedObjIdx
	if targetIdx < 0 || targetIdx >= len(objs) {
		targetIdx = len(objs) - 1
	}

	currObj := objs[targetIdx]

	typeEntry := widget.NewEntry()
	typeEntry.SetText(currObj.Type)
	wEntry := widget.NewEntry()
	wEntry.SetText(strconv.Itoa(currObj.Width))
	hEntry := widget.NewEntry()
	hEntry.SetText(strconv.Itoa(currObj.Height))
	attrsEntry := widget.NewEntry()
	attrsEntry.SetText(strings.Join(currObj.Attributes, ", "))
	xEntry := widget.NewEntry()
	xEntry.SetText(strconv.Itoa(currObj.Coordinates.X))
	yEntry := widget.NewEntry()
	yEntry.SetText(strconv.Itoa(currObj.Coordinates.Y))
	spriteSheetEntry := widget.NewEntry()
	spriteSheetEntry.SetText(currObj.SpriteSheet)
	subWEntry := widget.NewEntry()
	if currObj.SubSpriteWidth > 0 {
		subWEntry.SetText(strconv.Itoa(currObj.SubSpriteWidth))
	}
	subHEntry := widget.NewEntry()
	if currObj.SubSpriteHeight > 0 {
		subHEntry.SetText(strconv.Itoa(currObj.SubSpriteHeight))
	}

	form := widget.NewForm(
		widget.NewFormItem("Type", typeEntry),
		widget.NewFormItem("Width", wEntry),
		widget.NewFormItem("Height", hEntry),
		widget.NewFormItem("Attributes (comma-separated)", attrsEntry),
		widget.NewFormItem("X", xEntry),
		widget.NewFormItem("Y", yEntry),
		widget.NewFormItem("Sprite Sheet Path (optional)", spriteSheetEntry),
		widget.NewFormItem("Sub-Sprite Width (optional)", subWEntry),
		widget.NewFormItem("Sub-Sprite Height (optional)", subHEntry),
	)

	dialog.ShowCustomConfirm("Edit Object", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		x, _ := strconv.Atoi(xEntry.Text)
		y, _ := strconv.Atoi(yEntry.Text)
		w, _ := strconv.Atoi(wEntry.Text)
		h, _ := strconv.Atoi(hEntry.Text)
		subW, _ := strconv.Atoi(subWEntry.Text)
		subH, _ := strconv.Atoi(subHEntry.Text)
		var attrs []string
		if attrsEntry.Text != "" {
			for _, a := range strings.Split(attrsEntry.Text, ",") {
				if trimmed := strings.TrimSpace(a); trimmed != "" {
					attrs = append(attrs, trimmed)
				}
			}
		}

		editedObj := editor.Object{
			Type:            typeEntry.Text,
			Width:           w,
			Height:          h,
			Attributes:      attrs,
			Coordinates:     editor.Coordinates{X: x, Y: y},
			SpriteSheet:     spriteSheetEntry.Text,
			SubSpriteWidth:  subW,
			SubSpriteHeight: subH,
		}

		if editedObj.SpriteSheet != "" {
			if err := editor.ProcessObjectSpriteSheet(&editedObj, e.baseDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to process sprite sheet: %w", err), e.window)
			}
		}

		e.recordUndo()
		if err := e.room.ObjectLayers[layerIdx].EditObject(targetIdx, editedObj); err != nil {
			dialog.ShowError(err, e.window)
			return
		}
		e.statusLabel.SetText(fmt.Sprintf("Edited object #%d (%s)", targetIdx, editedObj.Type))
		e.refreshObjectList()
		e.refreshPreview()
	}, e.window)
}

func (e *EditorApp) buildTilesetLayersTab() fyne.CanvasObject {
	e.paletteContainer = container.NewStack()
	e.tsLayerSelect = widget.NewSelect([]string{}, func(selected string) {
		e.selectedTileIdx = -1
		e.loadSelectedTilesetLayerInfo()
		e.refreshTileList()
	})

	addTsLayerBtn := widget.NewButton("Add Tileset Layer", func() {
		e.room.TilesetLayers = append(e.room.TilesetLayers, editor.TilesetLayer{
			Name:        fmt.Sprintf("Tileset Layer %d", len(e.room.TilesetLayers)+1),
			TilesetPath: "",
			Attributes:  []string{},
			TileWidth:   16,
			TileHeight:  16,
			Tiles:       []editor.Tile{},
		})
		e.refreshTilesetLayerSelect()
	})

	removeTsLayerBtn := widget.NewButton("Delete Layer", func() {
		idx := e.tsLayerSelect.SelectedIndex()
		if idx < 0 || idx >= len(e.room.TilesetLayers) {
			return
		}
		e.room.TilesetLayers = append(e.room.TilesetLayers[:idx], e.room.TilesetLayers[idx+1:]...)
		e.refreshTilesetLayerSelect()
	})

	layerHeader := container.NewHBox(widget.NewLabel("Layer:"), e.tsLayerSelect, addTsLayerBtn, removeTsLayerBtn)

	e.tsPathEntry = widget.NewEntry()
	e.tsAttrEntry = widget.NewEntry()
	e.tileWEntry = widget.NewEntry()
	e.tileHEntry = widget.NewEntry()

	applyTsBtn := widget.NewButton("Apply Tileset Properties", func() {
		idx := e.tsLayerSelect.SelectedIndex()
		if idx < 0 || idx >= len(e.room.TilesetLayers) {
			return
		}
		tw, _ := strconv.Atoi(e.tileWEntry.Text)
		th, _ := strconv.Atoi(e.tileHEntry.Text)
		var attrs []string
		if e.tsAttrEntry.Text != "" {
			for _, a := range strings.Split(e.tsAttrEntry.Text, ",") {
				attrs = append(attrs, strings.TrimSpace(a))
			}
		}

		e.room.TilesetLayers[idx].TilesetPath = e.tsPathEntry.Text
		e.room.TilesetLayers[idx].TileWidth = tw
		e.room.TilesetLayers[idx].TileHeight = th
		e.room.TilesetLayers[idx].Attributes = attrs

		e.rebuildPalette()
		e.refreshPreview()
	})

	tsForm := widget.NewForm(
		widget.NewFormItem("Tileset Path", e.tsPathEntry),
		widget.NewFormItem("Tile Width", e.tileWEntry),
		widget.NewFormItem("Tile Height", e.tileHEntry),
		widget.NewFormItem("Attributes (comma-separated)", e.tsAttrEntry),
	)

	e.toolSelect = widget.NewRadioGroup([]string{"Brush Tool", "Selection Tool", "Place Tool"}, func(selected string) {
		switch selected {
		case "Brush Tool":
			e.activeTool = ToolBrush
		case "Selection Tool":
			e.activeTool = ToolSelection
		default:
			e.activeTool = ToolPlace
		}
	})
	e.toolSelect.SetSelected("Brush Tool")
	e.toolSelect.Horizontal = true

	toolsCard := widget.NewCard("Tileset Tools", "", e.toolSelect)
	tsPropertiesCard := widget.NewCard("Tileset Layer Settings", "", container.NewVBox(tsForm, applyTsBtn))
	paletteCard := widget.NewCard("Tile Set Palette", "Click a tile to select it for placing on room grid", e.paletteContainer)

	e.tileList = widget.NewList(
		func() int {
			idx := e.tsLayerSelect.SelectedIndex()
			if idx < 0 || idx >= len(e.room.TilesetLayers) {
				return 0
			}
			return len(e.room.TilesetLayers[idx].Tiles)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Tile Info")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			idx := e.tsLayerSelect.SelectedIndex()
			if idx >= 0 && idx < len(e.room.TilesetLayers) {
				tile := e.room.TilesetLayers[idx].Tiles[id]
				posStr := "Grid Position"
				if tile.Coordinates != nil {
					posStr = fmt.Sprintf("(%d, %d)", tile.Coordinates.X, tile.Coordinates.Y)
				}
				item.(*widget.Label).SetText(fmt.Sprintf("[%d] Tile Index: %s | Pos: %s", id, tile.Index, posStr))
			}
		},
	)
	e.tileList.OnSelected = func(id widget.ListItemID) {
		e.selectedTileIdx = id
	}

	addTileBtn := widget.NewButton("Add Tile", func() {
		layerIdx := e.tsLayerSelect.SelectedIndex()
		if layerIdx < 0 || layerIdx >= len(e.room.TilesetLayers) {
			dialog.ShowError(fmt.Errorf("select a tileset layer first"), e.window)
			return
		}

		idxEntry := widget.NewEntry()
		xEntry := widget.NewEntry()
		yEntry := widget.NewEntry()

		form := widget.NewForm(
			widget.NewFormItem("Tile Index in Asset", idxEntry),
			widget.NewFormItem("X (optional, empty for auto)", xEntry),
			widget.NewFormItem("Y (optional, empty for auto)", yEntry),
		)

		dialog.ShowCustomConfirm("Add Tile", "Add", "Cancel", form, func(ok bool) {
			if !ok {
				return
			}
			tile := editor.Tile{Index: idxEntry.Text}
			if xEntry.Text != "" && yEntry.Text != "" {
				x, _ := strconv.Atoi(xEntry.Text)
				y, _ := strconv.Atoi(yEntry.Text)
				tile.Coordinates = &editor.Coordinates{X: x, Y: y}
			}
			e.recordUndo()
			e.room.TilesetLayers[layerIdx].Tiles = append(e.room.TilesetLayers[layerIdx].Tiles, tile)
			e.refreshTileList()
			e.refreshPreview()
		}, e.window)
	})

	editTileBtn := widget.NewButton("Edit Selected Tile", func() {
		e.editSelectedTileDialog()
	})

	deleteTileBtn := widget.NewButton("Delete Selected Tile", func() {
		layerIdx := e.tsLayerSelect.SelectedIndex()
		if layerIdx < 0 || layerIdx >= len(e.room.TilesetLayers) {
			return
		}
		tiles := e.room.TilesetLayers[layerIdx].Tiles
		if len(tiles) == 0 {
			return
		}
		targetIdx := e.selectedTileIdx
		if targetIdx < 0 || targetIdx >= len(tiles) {
			targetIdx = len(tiles) - 1
		}
		e.recordUndo()
		e.room.TilesetLayers[layerIdx].Tiles = append(tiles[:targetIdx], tiles[targetIdx+1:]...)
		e.selectedTileIdx = -1
		e.refreshTileList()
		e.refreshPreview()
	})

	tileControls := container.NewHBox(addTileBtn, editTileBtn, deleteTileBtn)
	tilesCard := widget.NewCard("Tiles", "", container.NewBorder(nil, tileControls, nil, nil, e.tileList))

	topSection := container.NewVBox(layerHeader, toolsCard, tsPropertiesCard, paletteCard)
	return container.NewBorder(topSection, nil, nil, nil, tilesCard)
}

func (e *EditorApp) getCachedTilesetImage(layerTilesetPath string) (image.Image, error) {
	if layerTilesetPath == "" {
		return nil, fmt.Errorf("tileset_path is empty")
	}
	tsPath := layerTilesetPath
	if !filepath.IsAbs(tsPath) && e.baseDir != "" {
		tsPath = filepath.Join(e.baseDir, tsPath)
	}

	if e.tilesetCache == nil {
		e.tilesetCache = make(map[string]image.Image)
	}

	if img, ok := e.tilesetCache[tsPath]; ok {
		return img, nil
	}

	f, err := os.Open(tsPath)
	if err != nil {
		return nil, fmt.Errorf("tileset asset not found at path %q: %w", layerTilesetPath, err)
	}
	defer f.Close()

	tsImg, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tileset image %q: %w", layerTilesetPath, err)
	}

	e.tilesetCache[tsPath] = tsImg
	return tsImg, nil
}

func renderTilesetLayerCached(layer *editor.TilesetLayer, roomWidth, roomHeight int, tsImg image.Image) image.Image {
	tileW := layer.TileWidth
	tileH := layer.TileHeight
	if tileW <= 0 || tileH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	bounds := tsImg.Bounds()
	tsWidth := bounds.Dx()
	tsHeight := bounds.Dy()
	cols := tsWidth / tileW
	if cols <= 0 {
		cols = 1
	}

	dstWidth := roomWidth
	dstHeight := roomHeight
	if dstWidth <= 0 {
		dstWidth = 1
	}
	if dstHeight <= 0 {
		dstHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))

	colsInRoom := roomWidth / tileW
	if colsInRoom <= 0 {
		colsInRoom = 1
	}

	for i, tile := range layer.Tiles {
		tileIdx, err := strconv.Atoi(tile.Index)
		if err != nil {
			continue
		}

		srcX := (tileIdx % cols) * tileW
		srcY := (tileIdx / cols) * tileH

		if srcX < 0 || srcY < 0 || srcX+tileW > tsWidth || srcY+tileH > tsHeight {
			continue
		}

		var dstX, dstY int
		if tile.Coordinates != nil {
			dstX = tile.Coordinates.X
			dstY = tile.Coordinates.Y
		} else {
			col := i % colsInRoom
			row := i / colsInRoom
			dstX = col * tileW
			dstY = row * tileH
		}

		srcRect := image.Rect(srcX, srcY, srcX+tileW, srcY+tileH)
		dstRect := image.Rect(dstX, dstY, dstX+tileW, dstY+tileH)

		draw.Draw(dst, dstRect, tsImg, srcRect.Min, draw.Over)
	}

	return dst
}

func (e *EditorApp) renderRoomCompositeFast() (image.Image, error) {
	width := e.room.Width
	height := e.room.Height
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	composite := image.NewRGBA(image.Rect(0, 0, width, height))

	for _, layer := range e.room.TilesetLayers {
		if layer.TilesetPath == "" {
			continue
		}
		tsImg, err := e.getCachedTilesetImage(layer.TilesetPath)
		if err != nil {
			return nil, err
		}
		layerImg := renderTilesetLayerCached(&layer, width, height, tsImg)
		draw.Draw(composite, composite.Bounds(), layerImg, image.Point{}, draw.Over)
	}

	return composite, nil
}

func (e *EditorApp) rebuildPalette() {
	e.paletteButtons = make(map[string]*widget.Button)
	if e.tsLayerSelect == nil || e.paletteContainer == nil {
		return
	}
	idx := e.tsLayerSelect.SelectedIndex()
	if idx < 0 || idx >= len(e.room.TilesetLayers) {
		e.paletteContainer.Objects = []fyne.CanvasObject{widget.NewLabel("No tileset layer selected.")}
		e.paletteContainer.Refresh()
		return
	}

	layer := e.room.TilesetLayers[idx]
	if layer.TilesetPath == "" {
		e.paletteContainer.Objects = []fyne.CanvasObject{widget.NewLabel("No tileset_path specified for current layer.")}
		e.paletteContainer.Refresh()
		return
	}

	tsImg, err := e.getCachedTilesetImage(layer.TilesetPath)
	if err != nil {
		e.paletteContainer.Objects = []fyne.CanvasObject{widget.NewLabel(fmt.Sprintf("Failed to load tileset image: %v", err))}
		e.paletteContainer.Refresh()
		return
	}

	tileW := layer.TileWidth
	tileH := layer.TileHeight
	if tileW <= 0 || tileH <= 0 {
		e.paletteContainer.Objects = []fyne.CanvasObject{widget.NewLabel("Invalid tile width or height.")}
		e.paletteContainer.Refresh()
		return
	}

	tsBounds := tsImg.Bounds()
	cols := tsBounds.Dx() / tileW
	rows := tsBounds.Dy() / tileH
	total := cols * rows

	if cols <= 0 || total <= 0 {
		e.paletteContainer.Objects = []fyne.CanvasObject{widget.NewLabel("Tileset asset dimensions smaller than tile size.")}
		e.paletteContainer.Refresh()
		return
	}

	var items []fyne.CanvasObject
	for i := 0; i < total; i++ {
		srcX := (i % cols) * tileW
		srcY := (i / cols) * tileH

		subImg := image.NewRGBA(image.Rect(0, 0, tileW, tileH))
		draw.Draw(subImg, subImg.Bounds(), tsImg, image.Pt(srcX, srcY), draw.Src)

		cImg := canvas.NewImageFromImage(subImg)
		cImg.SetMinSize(fyne.NewSize(32, 32))
		cImg.FillMode = canvas.ImageFillContain

		tileIdxStr := strconv.Itoa(i)
		btn := widget.NewButton(fmt.Sprintf("#%s", tileIdxStr), nil)
		btn.OnTapped = func() {
			e.selectedPaletteTileIdx = tileIdxStr
			e.statusLabel.SetText(fmt.Sprintf("Selected palette tile #%s", tileIdxStr))
			e.updatePaletteHighlight()
		}

		e.paletteButtons[tileIdxStr] = btn

		item := container.NewVBox(cImg, btn)
		items = append(items, item)
	}

	if e.selectedPaletteTileIdx == "" {
		e.selectedPaletteTileIdx = "0"
	}

	gridCols := cols
	if gridCols > 8 {
		gridCols = 8
	}
	if gridCols <= 0 {
		gridCols = 1
	}
	grid := container.NewGridWithColumns(gridCols, items...)
	scroll := container.NewScroll(grid)
	scroll.SetMinSize(fyne.NewSize(0, 150))

	e.paletteContainer.Objects = []fyne.CanvasObject{scroll}
	e.paletteContainer.Refresh()
	e.updatePaletteHighlight()
}

func (e *EditorApp) updatePaletteHighlight() {
	for idxStr, btn := range e.paletteButtons {
		if idxStr == e.selectedPaletteTileIdx {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Refresh()
	}
}

func (e *EditorApp) refreshUI() {
	e.roomWidthEntry.SetText(strconv.Itoa(e.room.Width))
	e.roomHeightEntry.SetText(strconv.Itoa(e.room.Height))
	e.refreshObjectLayerSelect()
	e.refreshTilesetLayerSelect()
	e.refreshPreview()
}

func (e *EditorApp) refreshObjectLayerSelect() {
	var options []string
	for i, l := range e.room.ObjectLayers {
		name := l.Name
		if name == "" {
			name = fmt.Sprintf("Object Layer %d", i+1)
		}
		options = append(options, name)
	}
	e.objLayerSelect.SetOptions(options)
	if len(options) > 0 {
		e.objLayerSelect.SetSelectedIndex(0)
	} else {
		e.objLayerSelect.ClearSelected()
	}
	e.refreshObjectList()
}

func (e *EditorApp) refreshObjectList() {
	if e.objList != nil {
		e.objList.Refresh()
	}
}

func (e *EditorApp) refreshTilesetLayerSelect() {
	var options []string
	for i, l := range e.room.TilesetLayers {
		name := l.Name
		if name == "" {
			name = fmt.Sprintf("Tileset Layer %d", i+1)
		}
		options = append(options, name)
	}
	e.tsLayerSelect.SetOptions(options)
	if len(options) > 0 {
		e.tsLayerSelect.SetSelectedIndex(0)
	} else {
		e.tsLayerSelect.ClearSelected()
	}
	e.loadSelectedTilesetLayerInfo()
	e.refreshTileList()
}

func (e *EditorApp) loadSelectedTilesetLayerInfo() {
	idx := e.tsLayerSelect.SelectedIndex()
	if idx < 0 || idx >= len(e.room.TilesetLayers) {
		e.tsPathEntry.SetText("")
		e.tsAttrEntry.SetText("")
		e.tileWEntry.SetText("16")
		e.tileHEntry.SetText("16")
		e.rebuildPalette()
		return
	}
	layer := e.room.TilesetLayers[idx]
	e.tsPathEntry.SetText(layer.TilesetPath)
	e.tsAttrEntry.SetText(strings.Join(layer.Attributes, ", "))
	e.tileWEntry.SetText(strconv.Itoa(layer.TileWidth))
	e.tileHEntry.SetText(strconv.Itoa(layer.TileHeight))
	e.rebuildPalette()
}

func (e *EditorApp) refreshTileList() {
	if e.tileList != nil {
		e.tileList.Refresh()
	}
}

func drawObjectLetter(img *image.RGBA, letter string, x, y, w, h int, clr color.RGBA) {
	if w <= 0 {
		w = 12
	}
	if h <= 0 {
		h = 14
	}
	bgBox := image.Rect(x, y, x+w, y+h)
	draw.Draw(img, bgBox, &image.Uniform{color.RGBA{0, 0, 0, 200}}, image.Point{}, draw.Over)

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x + 2), Y: fixed.I(y + 11)},
	}
	d.DrawString(letter)
}

type previewOverlay struct {
	widget.BaseWidget
	onTap          func(pos fyne.Position, size fyne.Size)
	onSecondaryTap func(pos fyne.Position, size fyne.Size)
	onDrag         func(pos fyne.Position, size fyne.Size)
	onDragEnd      func()
}

func newPreviewOverlay(
	onTap func(pos fyne.Position, size fyne.Size),
	onSecondaryTap func(pos fyne.Position, size fyne.Size),
	onDrag func(pos fyne.Position, size fyne.Size),
	onDragEnd func(),
) *previewOverlay {
	po := &previewOverlay{
		onTap:          onTap,
		onSecondaryTap: onSecondaryTap,
		onDrag:         onDrag,
		onDragEnd:      onDragEnd,
	}
	po.ExtendBaseWidget(po)
	return po
}

func (po *previewOverlay) SecondaryTapped(e *fyne.PointEvent) {
	if po.onSecondaryTap != nil {
		po.onSecondaryTap(e.Position, po.Size())
	}
}

func (po *previewOverlay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (po *previewOverlay) Tapped(e *fyne.PointEvent) {
	if po.onTap != nil {
		po.onTap(e.Position, po.Size())
	}
}

func (po *previewOverlay) Dragged(e *fyne.DragEvent) {
	if po.onDrag != nil {
		po.onDrag(e.Position, po.Size())
	}
}

func (po *previewOverlay) DragEnd() {
	if po.onDragEnd != nil {
		po.onDragEnd()
	}
}

func (e *EditorApp) convertPosToRoomCoords(pos fyne.Position, containerSize fyne.Size) (roomX, roomY int, ok bool) {
	roomW := e.room.Width
	roomH := e.room.Height
	if roomW <= 0 || roomH <= 0 || containerSize.Width <= 0 || containerSize.Height <= 0 {
		return 0, 0, false
	}

	roomAspect := float64(roomW) / float64(roomH)
	containerAspect := float64(containerSize.Width) / float64(containerSize.Height)

	var renderW, renderH float64
	var offsetX, offsetY float64

	if containerAspect > roomAspect {
		renderH = float64(containerSize.Height)
		renderW = renderH * roomAspect
		offsetX = (float64(containerSize.Width) - renderW) / 2
		offsetY = 0
	} else {
		renderW = float64(containerSize.Width)
		renderH = renderW / roomAspect
		offsetX = 0
		offsetY = (float64(containerSize.Height) - renderH) / 2
	}

	relX := float64(pos.X) - offsetX
	relY := float64(pos.Y) - offsetY

	if relX < 0 || relX >= renderW || relY < 0 || relY >= renderH {
		return 0, 0, false
	}

	rx := int(relX * (float64(roomW) / renderW))
	ry := int(relY * (float64(roomH) / renderH))
	return rx, ry, true
}

func (e *EditorApp) findObjectAt(roomX, roomY int) (layerIdx int, objIdx int, found bool) {
	for lIdx, layer := range e.room.ObjectLayers {
		for oIdx, obj := range layer.Objects {
			w := obj.Width
			h := obj.Height
			if w <= 0 {
				w = 16
			}
			if h <= 0 {
				h = 16
			}
			if roomX >= obj.Coordinates.X-4 && roomX <= obj.Coordinates.X+w+4 &&
				roomY >= obj.Coordinates.Y-4 && roomY <= obj.Coordinates.Y+h+4 {
				return lIdx, oIdx, true
			}
		}
	}
	return -1, -1, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (e *EditorApp) deleteSelectedItems() {
	if len(e.selectedTiles) == 0 && len(e.selectedObjects) == 0 {
		return
	}
	e.recordUndo()

	objsToRemove := make(map[int]map[int]bool)
	for _, ref := range e.selectedObjects {
		if objsToRemove[ref.LayerIdx] == nil {
			objsToRemove[ref.LayerIdx] = make(map[int]bool)
		}
		objsToRemove[ref.LayerIdx][ref.ObjIdx] = true
	}

	for lIdx, layer := range e.room.ObjectLayers {
		if toDel, ok := objsToRemove[lIdx]; ok {
			var newObjs []editor.Object
			for oIdx, obj := range layer.Objects {
				if !toDel[oIdx] {
					newObjs = append(newObjs, obj)
				}
			}
			e.room.ObjectLayers[lIdx].Objects = newObjs
		}
	}

	tilesToRemove := make(map[int]map[int]bool)
	for _, ref := range e.selectedTiles {
		if tilesToRemove[ref.LayerIdx] == nil {
			tilesToRemove[ref.LayerIdx] = make(map[int]bool)
		}
		tilesToRemove[ref.LayerIdx][ref.TileIdx] = true
	}

	for lIdx, layer := range e.room.TilesetLayers {
		if toDel, ok := tilesToRemove[lIdx]; ok {
			var newTiles []editor.Tile
			for tIdx, tile := range layer.Tiles {
				if !toDel[tIdx] {
					newTiles = append(newTiles, tile)
				}
			}
			e.room.TilesetLayers[lIdx].Tiles = newTiles
		}
	}

	e.selectedObjects = nil
	e.selectedTiles = nil
	e.hasSelectionBox = false
	e.statusLabel.SetText("Deleted selected tiles and objects")
	e.refreshObjectList()
	e.refreshTileList()
	e.refreshPreview()
}

func (e *EditorApp) updateSelectionFromRect(minX, maxX, minY, maxY int) {
	e.selectedObjects = nil
	e.selectedTiles = nil

	for lIdx, layer := range e.room.ObjectLayers {
		for oIdx, obj := range layer.Objects {
			if obj.Coordinates.X >= minX-8 && obj.Coordinates.X <= maxX+8 &&
				obj.Coordinates.Y >= minY-8 && obj.Coordinates.Y <= maxY+8 {
				e.selectedObjects = append(e.selectedObjects, SelectedObjRef{LayerIdx: lIdx, ObjIdx: oIdx})
			}
		}
	}

	for lIdx, layer := range e.room.TilesetLayers {
		tileW := layer.TileWidth
		tileH := layer.TileHeight
		if tileW <= 0 || tileH <= 0 {
			tileW, tileH = 16, 16
		}
		for tIdx, tile := range layer.Tiles {
			if tile.Coordinates != nil {
				tx := tile.Coordinates.X
				ty := tile.Coordinates.Y
				if tx+tileW >= minX && tx <= maxX && ty+tileH >= minY && ty <= maxY {
					e.selectedTiles = append(e.selectedTiles, SelectedTileRef{LayerIdx: lIdx, TileIdx: tIdx})
				}
			}
		}
	}

	e.statusLabel.SetText(fmt.Sprintf("Selection contains %d objects, %d tiles", len(e.selectedObjects), len(e.selectedTiles)))
}

func (e *EditorApp) handlePreviewTap(pos fyne.Position, containerSize fyne.Size) {
	roomX, roomY, ok := e.convertPosToRoomCoords(pos, containerSize)
	if !ok {
		return
	}

	if e.activeTool == ToolSelection {
		e.selectionStart = editor.Coordinates{X: roomX - 8, Y: roomY - 8}
		e.selectionEnd = editor.Coordinates{X: roomX + 8, Y: roomY + 8}
		e.hasSelectionBox = true
		e.updateSelectionFromRect(roomX-8, roomX+8, roomY-8, roomY+8)
		e.refreshPreview()
		return
	}

	lIdx, oIdx, foundObj := e.findObjectAt(roomX, roomY)
	if foundObj {
		if e.objLayerSelect != nil {
			e.objLayerSelect.SetSelectedIndex(lIdx)
		}
		e.selectedObjIdx = oIdx
		e.refreshObjectList()
		if e.objList != nil {
			e.objList.Select(oIdx)
		}
		obj := e.room.ObjectLayers[lIdx].Objects[oIdx]
		e.statusLabel.SetText(fmt.Sprintf("Selected object type %q at (%d, %d)", obj.Type, obj.Coordinates.X, obj.Coordinates.Y))
		return
	}

	e.placeTileAtRoomCoords(roomX, roomY)
}

func (e *EditorApp) handlePreviewDrag(pos fyne.Position, containerSize fyne.Size) {
	roomX, roomY, ok := e.convertPosToRoomCoords(pos, containerSize)
	if !ok {
		return
	}

	if !e.isDragging {
		if e.activeTool == ToolSelection {
			minX := min(e.selectionStart.X, e.selectionEnd.X)
			maxX := max(e.selectionStart.X, e.selectionEnd.X)
			minY := min(e.selectionStart.Y, e.selectionEnd.Y)
			maxY := max(e.selectionStart.Y, e.selectionEnd.Y)

			if e.hasSelectionBox && roomX >= minX && roomX <= maxX && roomY >= minY && roomY <= maxY {
				e.isMovingSelection = true
				e.moveStartCoords = editor.Coordinates{X: roomX, Y: roomY}
				e.recordUndo()
				e.isDragging = true
			} else {
				e.isSelecting = true
				e.selectionStart = editor.Coordinates{X: roomX, Y: roomY}
				e.selectionEnd = editor.Coordinates{X: roomX, Y: roomY}
				e.hasSelectionBox = true
				e.updateSelectionFromRect(roomX, roomX, roomY, roomY)
				e.isDragging = true
			}
		} else {
			lIdx, oIdx, foundObj := e.findObjectAt(roomX, roomY)
			if foundObj {
				e.draggedObjLayerIdx = lIdx
				e.draggedObjIdx = oIdx
				e.recordUndo()
				e.isDragging = true
			} else if e.activeTool == ToolBrush {
				e.draggedObjLayerIdx = -1
				e.draggedObjIdx = -1
				e.recordUndo()
				e.isDragging = true
			}
		}
	}

	if e.isMovingSelection {
		dx := roomX - e.moveStartCoords.X
		dy := roomY - e.moveStartCoords.Y
		if dx != 0 || dy != 0 {
			e.moveStartCoords = editor.Coordinates{X: roomX, Y: roomY}
			e.selectionStart.X += dx
			e.selectionStart.Y += dy
			e.selectionEnd.X += dx
			e.selectionEnd.Y += dy

			for _, ref := range e.selectedObjects {
				if ref.LayerIdx < len(e.room.ObjectLayers) && ref.ObjIdx < len(e.room.ObjectLayers[ref.LayerIdx].Objects) {
					e.room.ObjectLayers[ref.LayerIdx].Objects[ref.ObjIdx].Coordinates.X += dx
					e.room.ObjectLayers[ref.LayerIdx].Objects[ref.ObjIdx].Coordinates.Y += dy
				}
			}

			for _, ref := range e.selectedTiles {
				if ref.LayerIdx < len(e.room.TilesetLayers) && ref.TileIdx < len(e.room.TilesetLayers[ref.LayerIdx].Tiles) {
					t := &e.room.TilesetLayers[ref.LayerIdx].Tiles[ref.TileIdx]
					if t.Coordinates != nil {
						t.Coordinates.X += dx
						t.Coordinates.Y += dy
					}
				}
			}

			e.refreshPreview()
		}
	} else if e.isSelecting {
		e.selectionEnd = editor.Coordinates{X: roomX, Y: roomY}
		minX := min(e.selectionStart.X, e.selectionEnd.X)
		maxX := max(e.selectionStart.X, e.selectionEnd.X)
		minY := min(e.selectionStart.Y, e.selectionEnd.Y)
		maxY := max(e.selectionStart.Y, e.selectionEnd.Y)
		e.updateSelectionFromRect(minX, maxX, minY, maxY)
		e.refreshPreview()
	} else if e.draggedObjLayerIdx >= 0 && e.draggedObjIdx >= 0 {
		if e.draggedObjLayerIdx < len(e.room.ObjectLayers) && e.draggedObjIdx < len(e.room.ObjectLayers[e.draggedObjLayerIdx].Objects) {
			obj := &e.room.ObjectLayers[e.draggedObjLayerIdx].Objects[e.draggedObjIdx]
			obj.Coordinates = editor.Coordinates{X: roomX, Y: roomY}
			e.statusLabel.SetText(fmt.Sprintf("Moving object %q to (%d, %d)", obj.Type, roomX, roomY))
			e.refreshPreview()
		}
	} else if e.activeTool == ToolBrush {
		tileW := 16
		tileH := 16
		if e.tsLayerSelect != nil {
			if idx := e.tsLayerSelect.SelectedIndex(); idx >= 0 && idx < len(e.room.TilesetLayers) {
				if e.room.TilesetLayers[idx].TileWidth > 0 {
					tileW = e.room.TilesetLayers[idx].TileWidth
				}
				if e.room.TilesetLayers[idx].TileHeight > 0 {
					tileH = e.room.TilesetLayers[idx].TileHeight
				}
			}
		}
		gridX := (roomX / tileW) * tileW
		gridY := (roomY / tileH) * tileH
		if gridX != e.lastBrushX || gridY != e.lastBrushY {
			e.lastBrushX = gridX
			e.lastBrushY = gridY
			e.placeTileAtRoomCoordsInternal(roomX, roomY, false)
		}
	}
}

func (e *EditorApp) snapSelectedTilesToGrid() {
	for _, ref := range e.selectedTiles {
		if ref.LayerIdx >= 0 && ref.LayerIdx < len(e.room.TilesetLayers) {
			layer := &e.room.TilesetLayers[ref.LayerIdx]
			tileW := layer.TileWidth
			tileH := layer.TileHeight
			if tileW <= 0 || tileH <= 0 {
				tileW, tileH = 16, 16
			}
			if ref.TileIdx >= 0 && ref.TileIdx < len(layer.Tiles) {
				t := &layer.Tiles[ref.TileIdx]
				if t.Coordinates != nil {
					t.Coordinates.X = (t.Coordinates.X / tileW) * tileW
					t.Coordinates.Y = (t.Coordinates.Y / tileH) * tileH
				}
			}
		}
	}
}

func (e *EditorApp) editSelectedTileDialog() {
	layerIdx := e.tsLayerSelect.SelectedIndex()
	if layerIdx < 0 || layerIdx >= len(e.room.TilesetLayers) {
		dialog.ShowError(fmt.Errorf("select a tileset layer first"), e.window)
		return
	}
	tiles := e.room.TilesetLayers[layerIdx].Tiles
	if len(tiles) == 0 {
		dialog.ShowError(fmt.Errorf("no tiles in selected layer"), e.window)
		return
	}
	targetIdx := e.selectedTileIdx
	if targetIdx < 0 || targetIdx >= len(tiles) {
		targetIdx = len(tiles) - 1
	}

	currTile := tiles[targetIdx]

	idxEntry := widget.NewEntry()
	idxEntry.SetText(currTile.Index)
	xEntry := widget.NewEntry()
	yEntry := widget.NewEntry()
	if currTile.Coordinates != nil {
		xEntry.SetText(strconv.Itoa(currTile.Coordinates.X))
		yEntry.SetText(strconv.Itoa(currTile.Coordinates.Y))
	}

	form := widget.NewForm(
		widget.NewFormItem("Tile Index in Asset", idxEntry),
		widget.NewFormItem("X (optional, empty for auto)", xEntry),
		widget.NewFormItem("Y (optional, empty for auto)", yEntry),
	)

	dialog.ShowCustomConfirm("Edit Tile", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		editedTile := editor.Tile{Index: idxEntry.Text}
		if xEntry.Text != "" && yEntry.Text != "" {
			x, _ := strconv.Atoi(xEntry.Text)
			y, _ := strconv.Atoi(yEntry.Text)
			editedTile.Coordinates = &editor.Coordinates{X: x, Y: y}
		}
		e.recordUndo()
		if err := e.room.TilesetLayers[layerIdx].EditTile(targetIdx, editedTile); err != nil {
			dialog.ShowError(err, e.window)
			return
		}
		e.statusLabel.SetText(fmt.Sprintf("Edited tile #%d (index: %s)", targetIdx, editedTile.Index))
		e.refreshTileList()
		e.refreshPreview()
	}, e.window)
}

func (e *EditorApp) handlePreviewSecondaryTap(pos fyne.Position, containerSize fyne.Size) {
	var menuItems []*fyne.MenuItem

	roomX, roomY, ok := e.convertPosToRoomCoords(pos, containerSize)
	if ok {
		lIdx, oIdx, foundObj := e.findObjectAt(roomX, roomY)
		if foundObj {
			menuItems = append(menuItems, fyne.NewMenuItem("Edit Object at Click", func() {
				if e.objLayerSelect != nil {
					e.objLayerSelect.SetSelectedIndex(lIdx)
				}
				e.selectedObjIdx = oIdx
				e.refreshObjectList()
				if e.objList != nil {
					e.objList.Select(oIdx)
				}
				e.editSelectedObjectDialog()
			}))
		}
	}

	menuItems = append(menuItems,
		fyne.NewMenuItem("Edit Selected Object", func() {
			e.editSelectedObjectDialog()
		}),
		fyne.NewMenuItem("Edit Selected Tile", func() {
			e.editSelectedTileDialog()
		}),
		fyne.NewMenuItem("Delete Selected Items", func() {
			e.deleteSelectedItems()
		}),
		fyne.NewMenuItem("Move / Select Tool", func() {
			if e.toolSelect != nil {
				e.toolSelect.SetSelected("Selection Tool")
			}
		}),
		fyne.NewMenuItem("Brush Tool", func() {
			if e.toolSelect != nil {
				e.toolSelect.SetSelected("Brush Tool")
			}
		}),
		fyne.NewMenuItem("Export Room", func() {
			e.exportRoom()
		}),
		fyne.NewMenuItem("Save Room", func() {
			e.saveRoom()
		}),
		fyne.NewMenuItem("Undo", func() {
			e.undo()
		}),
		fyne.NewMenuItem("Redo", func() {
			e.redo()
		}),
	)

	menu := fyne.NewMenu("Actions", menuItems...)
	popUp := widget.NewPopUpMenu(menu, e.window.Canvas())
	popUp.ShowAtPosition(pos)
}

func (e *EditorApp) handlePreviewDragEnd() {
	if e.isMovingSelection {
		e.snapSelectedTilesToGrid()
	}
	e.refreshObjectList()
	e.refreshTileList()
	e.refreshPreview()

	e.isDragging = false
	e.isSelecting = false
	e.isMovingSelection = false
	e.draggedObjLayerIdx = -1
	e.draggedObjIdx = -1
	e.lastBrushX = -1
	e.lastBrushY = -1
}

func (e *EditorApp) selectTileAtRoomCoords(roomX, roomY int) {
	if e.tsLayerSelect == nil {
		return
	}
	layerIdx := e.tsLayerSelect.SelectedIndex()
	if layerIdx < 0 || layerIdx >= len(e.room.TilesetLayers) {
		return
	}

	layer := &e.room.TilesetLayers[layerIdx]
	tileW := layer.TileWidth
	tileH := layer.TileHeight
	if tileW <= 0 || tileH <= 0 {
		tileW = 16
		tileH = 16
	}

	gridX := (roomX / tileW) * tileW
	gridY := (roomY / tileH) * tileH

	for i, tile := range layer.Tiles {
		if tile.Coordinates != nil && tile.Coordinates.X == gridX && tile.Coordinates.Y == gridY {
			e.selectedTileIdx = i
			e.selectedPaletteTileIdx = tile.Index
			e.updatePaletteHighlight()
			e.refreshTileList()
			if e.tileList != nil {
				e.tileList.Select(i)
			}
			e.statusLabel.SetText(fmt.Sprintf("Selected tile #%s at (%d, %d)", tile.Index, gridX, gridY))
			return
		}
	}

	e.statusLabel.SetText(fmt.Sprintf("No tile found at grid (%d, %d)", gridX, gridY))
}

func (e *EditorApp) placeTileAtRoomCoords(roomX, roomY int) {
	e.placeTileAtRoomCoordsInternal(roomX, roomY, true)
}

func (e *EditorApp) placeTileAtRoomCoordsInternal(roomX, roomY int, recordHistory bool) {
	if e.tsLayerSelect == nil {
		return
	}
	layerIdx := e.tsLayerSelect.SelectedIndex()
	if layerIdx < 0 || layerIdx >= len(e.room.TilesetLayers) {
		e.statusLabel.SetText("Select a tileset layer first to place tiles")
		return
	}

	layer := &e.room.TilesetLayers[layerIdx]
	tileW := layer.TileWidth
	tileH := layer.TileHeight
	if tileW <= 0 || tileH <= 0 {
		tileW = 16
		tileH = 16
	}

	gridX := (roomX / tileW) * tileW
	gridY := (roomY / tileH) * tileH

	tileIdxStr := e.selectedPaletteTileIdx
	if tileIdxStr == "" {
		tileIdxStr = "0"
	}

	if recordHistory {
		e.recordUndo()
	}

	found := false
	for i := range layer.Tiles {
		if layer.Tiles[i].Coordinates != nil && layer.Tiles[i].Coordinates.X == gridX && layer.Tiles[i].Coordinates.Y == gridY {
			layer.Tiles[i].Index = tileIdxStr
			found = true
			break
		}
	}
	if !found {
		layer.Tiles = append(layer.Tiles, editor.Tile{
			Index:       tileIdxStr,
			Coordinates: &editor.Coordinates{X: gridX, Y: gridY},
		})
	}

	e.statusLabel.SetText(fmt.Sprintf("Placed tile #%s at (%d, %d)", tileIdxStr, gridX, gridY))
	e.refreshTileList()
	e.refreshPreview()
}

func (e *EditorApp) refreshPreview() {
	img, err := e.renderRoomCompositeFast()
	if err != nil {
		errMsg := fmt.Sprintf("Render Error: %v", err)
		e.statusLabel.SetText("Error rendering room preview")
		errCard := widget.NewCard("Render Failure / Missing Asset", "", widget.NewLabel(errMsg))
		e.previewContainer.Objects = []fyne.CanvasObject{errCard}
		e.previewContainer.Refresh()
		return
	}

	bounds := img.Bounds()
	rgbaImg := image.NewRGBA(bounds)
	draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)

	// Draw faint grid lines for the active tileset layer grid resolution
	if e.tsLayerSelect != nil {
		if idx := e.tsLayerSelect.SelectedIndex(); idx >= 0 && idx < len(e.room.TilesetLayers) {
			tsLayer := e.room.TilesetLayers[idx]
			tw, th := tsLayer.TileWidth, tsLayer.TileHeight
			if tw > 0 && th > 0 {
				gridColor := color.RGBA{128, 128, 128, 60}
				for x := tw; x < bounds.Dx(); x += tw {
					for y := 0; y < bounds.Dy(); y++ {
						rgbaImg.Set(x, y, gridColor)
					}
				}
				for y := th; y < bounds.Dy(); y += th {
					for x := 0; x < bounds.Dx(); x++ {
						rgbaImg.Set(x, y, gridColor)
					}
				}
			}
		}
	}

	// Draw selection box if active
	if e.hasSelectionBox {
		minX := min(e.selectionStart.X, e.selectionEnd.X)
		maxX := max(e.selectionStart.X, e.selectionEnd.X)
		minY := min(e.selectionStart.Y, e.selectionEnd.Y)
		maxY := max(e.selectionStart.Y, e.selectionEnd.Y)

		selRect := image.Rect(minX, minY, maxX+1, maxY+1).Intersect(bounds)
		if !selRect.Empty() {
			overlayColor := image.NewUniform(color.RGBA{255, 255, 255, 60})
			draw.Draw(rgbaImg, selRect, overlayColor, image.Point{}, draw.Over)

			for x := selRect.Min.X; x < selRect.Max.X; x++ {
				rgbaImg.Set(x, selRect.Min.Y, color.White)
				rgbaImg.Set(x, selRect.Max.Y-1, color.White)
			}
			for y := selRect.Min.Y; y < selRect.Max.Y; y++ {
				rgbaImg.Set(selRect.Min.X, y, color.White)
				rgbaImg.Set(selRect.Max.X-1, y, color.White)
			}
		}
	}

	// Draw objects on room preview: draw first sub-sprite if sprite sheet exists, else letter representation
	for _, layer := range e.room.ObjectLayers {
		for _, obj := range layer.Objects {
			drawnSprite := false
			if obj.SpriteSheet != "" && len(obj.Sprites) > 0 {
				subImg, err := editor.LoadSubSprite(&obj, 0, e.baseDir)
				if err == nil {
					w := obj.Width
					h := obj.Height
					if w <= 0 {
						w = obj.SubSpriteWidth
					}
					if h <= 0 {
						h = obj.SubSpriteHeight
					}
					dstRect := image.Rect(obj.Coordinates.X, obj.Coordinates.Y, obj.Coordinates.X+w, obj.Coordinates.Y+h)
					draw.Draw(rgbaImg, dstRect, subImg, image.Point{}, draw.Over)
					drawnSprite = true
				}
			}

			if !drawnSprite {
				if len(obj.Type) == 0 {
					continue
				}
				runes := []rune(obj.Type)
				letter := string(runes[0])

				// Generate random letter color every time
				clr := color.RGBA{
					R: uint8(rand.Intn(256)),
					G: uint8(rand.Intn(256)),
					B: uint8(rand.Intn(256)),
					A: 255,
				}

				drawObjectLetter(rgbaImg, letter, obj.Coordinates.X, obj.Coordinates.Y, obj.Width, obj.Height, clr)
			}
		}
	}

	e.statusLabel.SetText("Preview updated")
	canvasImg := canvas.NewImageFromImage(rgbaImg)
	canvasImg.FillMode = canvas.ImageFillContain

	// Background rectangle
	bg := canvas.NewRectangle(color.RGBA{R: 30, G: 30, B: 30, A: 255})

	overlay := newPreviewOverlay(
		func(pos fyne.Position, size fyne.Size) {
			e.handlePreviewTap(pos, size)
		},
		func(pos fyne.Position, size fyne.Size) {
			e.handlePreviewSecondaryTap(pos, size)
		},
		func(pos fyne.Position, size fyne.Size) {
			e.handlePreviewDrag(pos, size)
		},
		func() {
			e.handlePreviewDragEnd()
		},
	)

	e.previewContainer.Objects = []fyne.CanvasObject{
		bg,
		canvasImg,
		overlay,
	}
	e.previewContainer.Refresh()
}

func (e *EditorApp) newRoom() {
	e.room = &editor.Room{
		Width:         640,
		Height:        480,
		ObjectLayers:  []editor.ObjectLayer{},
		TilesetLayers: []editor.TilesetLayer{},
	}
	e.currentPath = ""
	e.refreshUI()
	e.statusLabel.SetText("New room created")
}

func (e *EditorApp) openRoom() {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		filePath := reader.URI().Path()
		rm, err := editor.LoadRoomFromFile(filePath)
		if err != nil {
			dialog.ShowError(err, e.window)
			return
		}

		e.room = rm
		e.currentPath = filePath
		e.baseDir = filepath.Dir(filePath)

		for lIdx := range e.room.ObjectLayers {
			for oIdx := range e.room.ObjectLayers[lIdx].Objects {
				obj := &e.room.ObjectLayers[lIdx].Objects[oIdx]
				if obj.SpriteSheet != "" && len(obj.Sprites) == 0 {
					_ = editor.ProcessObjectSpriteSheet(obj, e.baseDir)
				}
			}
		}

		e.refreshUI()
		e.statusLabel.SetText("Opened " + filePath)
	}, e.window)

	d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	d.Show()
}

func (e *EditorApp) saveRoom() {
	if e.currentPath == "" {
		e.exportRoom()
		return
	}
	if err := editor.SaveRoomToFile(e.room, e.currentPath); err != nil {
		dialog.ShowError(err, e.window)
		return
	}
	e.statusLabel.SetText("Saved " + e.currentPath)
}

func (e *EditorApp) exportRoom() {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		filePath := writer.URI().Path()
		if err := editor.SaveRoomToFile(e.room, filePath); err != nil {
			dialog.ShowError(err, e.window)
			return
		}

		e.currentPath = filePath
		e.baseDir = filepath.Dir(filePath)
		e.statusLabel.SetText("Exported to " + filePath)
		dialog.ShowInformation("Export Room", "Room successfully exported to JSON!", e.window)
	}, e.window)

	d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	d.SetFileName("room.json")
	d.Show()
}

func main() {
	editorApp := newEditorApp()
	content := editorApp.buildUI()
	editorApp.window.SetContent(content)
	editorApp.window.ShowAndRun()
}
