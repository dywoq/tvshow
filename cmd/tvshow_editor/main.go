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
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"tvshow/game/editor"
)

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
		selectedObjIdx:  -1,
		selectedTileIdx: -1,
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
	)

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
				item.(*widget.Label).SetText(fmt.Sprintf("[%d] Type: %s | Pos: (%d, %d) | Attrs: %v",
					id, obj.Type, obj.Coordinates.X, obj.Coordinates.Y, obj.Attributes))
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
		attrsEntry := widget.NewEntry()
		xEntry := widget.NewEntry()
		xEntry.SetText("0")
		yEntry := widget.NewEntry()
		yEntry.SetText("0")

		form := widget.NewForm(
			widget.NewFormItem("Type", typeEntry),
			widget.NewFormItem("Attributes (comma-separated)", attrsEntry),
			widget.NewFormItem("X", xEntry),
			widget.NewFormItem("Y", yEntry),
		)

		dialog.ShowCustomConfirm("Add Object", "Add", "Cancel", form, func(ok bool) {
			if !ok {
				return
			}
			x, _ := strconv.Atoi(xEntry.Text)
			y, _ := strconv.Atoi(yEntry.Text)
			var attrs []string
			if attrsEntry.Text != "" {
				for _, a := range strings.Split(attrsEntry.Text, ",") {
					attrs = append(attrs, strings.TrimSpace(a))
				}
			}

			newObj := editor.Object{
				Type:        typeEntry.Text,
				Attributes:  attrs,
				Coordinates: editor.Coordinates{X: x, Y: y},
			}
			e.room.ObjectLayers[layerIdx].Objects = append(e.room.ObjectLayers[layerIdx].Objects, newObj)
			e.refreshObjectList()
		}, e.window)
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
		e.room.ObjectLayers[layerIdx].Objects = append(objs[:targetIdx], objs[targetIdx+1:]...)
		e.selectedObjIdx = -1
		e.refreshObjectList()
	})

	objControls := container.NewHBox(addObjBtn, deleteObjBtn)

	return container.NewBorder(layerHeader, objControls, nil, nil, e.objList)
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
			e.room.TilesetLayers[layerIdx].Tiles = append(e.room.TilesetLayers[layerIdx].Tiles, tile)
			e.refreshTileList()
			e.refreshPreview()
		}, e.window)
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
		e.room.TilesetLayers[layerIdx].Tiles = append(tiles[:targetIdx], tiles[targetIdx+1:]...)
		e.selectedTileIdx = -1
		e.refreshTileList()
		e.refreshPreview()
	})

	tileControls := container.NewHBox(addTileBtn, deleteTileBtn)
	tilesCard := widget.NewCard("Tiles", "", container.NewBorder(nil, tileControls, nil, nil, e.tileList))

	topSection := container.NewVBox(layerHeader, tsPropertiesCard, paletteCard)
	return container.NewBorder(topSection, nil, nil, nil, tilesCard)
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

	tsPath := layer.TilesetPath
	if !filepath.IsAbs(tsPath) && e.baseDir != "" {
		tsPath = filepath.Join(e.baseDir, tsPath)
	}

	f, err := os.Open(tsPath)
	if err != nil {
		e.paletteContainer.Objects = []fyne.CanvasObject{widget.NewLabel(fmt.Sprintf("Tileset asset not found: %v", err))}
		e.paletteContainer.Refresh()
		return
	}
	defer f.Close()

	tsImg, _, err := image.Decode(f)
	if err != nil {
		e.paletteContainer.Objects = []fyne.CanvasObject{widget.NewLabel(fmt.Sprintf("Failed to decode tileset image: %v", err))}
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

func drawObjectLetter(img *image.RGBA, letter string, x, y int, clr color.RGBA) {
	bgBox := image.Rect(x, y, x+10, y+14)
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
	onTap func(pos fyne.Position, size fyne.Size)
}

func newPreviewOverlay(onTap func(pos fyne.Position, size fyne.Size)) *previewOverlay {
	po := &previewOverlay{onTap: onTap}
	po.ExtendBaseWidget(po)
	return po
}

func (po *previewOverlay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (po *previewOverlay) Tapped(e *fyne.PointEvent) {
	if po.onTap != nil {
		po.onTap(e.Position, po.Size())
	}
}

func (e *EditorApp) handlePreviewTap(pos fyne.Position, containerSize fyne.Size) {
	roomW := e.room.Width
	roomH := e.room.Height
	if roomW <= 0 || roomH <= 0 || containerSize.Width <= 0 || containerSize.Height <= 0 {
		return
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
		return
	}

	roomX := int(relX * (float64(roomW) / renderW))
	roomY := int(relY * (float64(roomH) / renderH))

	e.placeTileAtRoomCoords(roomX, roomY)
}

func (e *EditorApp) placeTileAtRoomCoords(roomX, roomY int) {
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
	img, err := editor.RenderRoomComposite(e.room, e.baseDir)
	if err != nil {
		// Report tileset error explicitly
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

	// Draw objects on room preview: shown as the first letter of their type with a random color every time
	for _, layer := range e.room.ObjectLayers {
		for _, obj := range layer.Objects {
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

			drawObjectLetter(rgbaImg, letter, obj.Coordinates.X, obj.Coordinates.Y, clr)
		}
	}

	e.statusLabel.SetText("Preview updated")
	canvasImg := canvas.NewImageFromImage(rgbaImg)
	canvasImg.FillMode = canvas.ImageFillContain

	// Background rectangle
	bg := canvas.NewRectangle(color.RGBA{R: 30, G: 30, B: 30, A: 255})

	overlay := newPreviewOverlay(func(pos fyne.Position, size fyne.Size) {
		e.handlePreviewTap(pos, size)
	})

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
