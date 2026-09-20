package editor

import (
	"encoding/json"
	"fmt"

	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
)

// Coordinates represents x and y coordinates in a room.
type Coordinates struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// SubSprite represents sprite information within an object's sprite sheet.
type SubSprite struct {
	Index int `json:"index"`
}

// Object represents a room object within an object layer.
type Object struct {
	Type                 string      `json:"type"`
	Attributes           []string    `json:"attributes"`
	Width                int         `json:"width"`
	Height               int         `json:"height"`
	Coordinates          Coordinates `json:"coordinates"`
	SpriteSheet          string      `json:"sprite_sheet,omitempty"`
	SubSpriteWidth       int         `json:"sub_sprite_width,omitempty"`
	SubSpriteHeight      int         `json:"sub_sprite_height,omitempty"`
	SubSpritesTotalCount int         `json:"sub_sprites_total_count,omitempty"`
	Sprites              []SubSprite `json:"sprites,omitempty"`
}

// ObjectLayer represents a layer containing objects.
type ObjectLayer struct {
	Name       string   `json:"name,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
	Objects    []Object `json:"objects,omitempty"`
}

// Tile represents a tile within a tileset layer.
type Tile struct {
	Index       string       `json:"index"`
	Coordinates *Coordinates `json:"coordinates,omitempty"`
}

// TilesetLayer represents a layer in the tileset group.
type TilesetLayer struct {
	Name        string   `json:"name,omitempty"`
	TilesetPath string   `json:"tileset_path"`
	Attributes  []string `json:"attributes"`
	TileWidth   int      `json:"tile_width"`
	TileHeight  int      `json:"tile_height"`
	Tiles       []Tile   `json:"tiles"`
}

// Room represents the room JSON structure.
type Room struct {
	Width         int            `json:"width"`
	Height        int            `json:"height"`
	ObjectLayers  []ObjectLayer  `json:"object_layers"`
	TilesetLayers []TilesetLayer `json:"tileset_layers"`
}

// LoadRoomFromBytes parses a Room from JSON bytes.
func LoadRoomFromBytes(data []byte) (*Room, error) {
	var room Room
	if err := json.Unmarshal(data, &room); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room JSON: %w", err)
	}
	if room.ObjectLayers == nil {
		room.ObjectLayers = []ObjectLayer{}
	}
	if room.TilesetLayers == nil {
		room.TilesetLayers = []TilesetLayer{}
	}
	return &room, nil
}

// LoadRoomFromFile reads a JSON room file from path and parses it.
func LoadRoomFromFile(path string) (*Room, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read room file: %w", err)
	}
	return LoadRoomFromBytes(data)
}

// SaveRoomToBytes serializes a Room into JSON bytes.
func SaveRoomToBytes(room *Room) ([]byte, error) {
	data, err := json.MarshalIndent(room, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal room: %w", err)
	}
	return data, nil
}

// SaveRoomToFile serializes a Room and writes it to a file.
func SaveRoomToFile(room *Room, path string) error {
	data, err := SaveRoomToBytes(room)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write room file: %w", err)
	}
	return nil
}

// RenderTilesetLayer renders a single tileset layer onto a new image.Image of room size (roomWidth x roomHeight).
// baseDir is used to resolve relative tileset_path.
func RenderTilesetLayer(layer *TilesetLayer, roomWidth, roomHeight int, baseDir string) (image.Image, error) {
	if layer.TilesetPath == "" {
		return nil, fmt.Errorf("tileset_path is empty")
	}

	tilesetPath := layer.TilesetPath
	if !filepath.IsAbs(tilesetPath) && baseDir != "" {
		tilesetPath = filepath.Join(baseDir, tilesetPath)
	}

	f, err := os.Open(tilesetPath)
	if err != nil {
		return nil, fmt.Errorf("tileset asset not found at path %q: %w", layer.TilesetPath, err)
	}
	defer f.Close()

	tilesetImg, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tileset image %q: %w", layer.TilesetPath, err)
	}

	tileW := layer.TileWidth
	tileH := layer.TileHeight
	if tileW <= 0 || tileH <= 0 {
		return nil, fmt.Errorf("invalid tile dimensions: %dx%d", tileW, tileH)
	}

	bounds := tilesetImg.Bounds()
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
			// If index is not an integer string, skip or handle error
			continue
		}

		srcX := (tileIdx % cols) * tileW
		srcY := (tileIdx / cols) * tileH

		if srcX < 0 || srcY < 0 || srcX+tileW > tsWidth || srcY+tileH > tsHeight {
			// Index out of bounds of tileset image
			continue
		}

		var dstX, dstY int
		if tile.Coordinates != nil {
			dstX = tile.Coordinates.X
			dstY = tile.Coordinates.Y
		} else {
			// Calculate grid coordinates based on array index
			col := i % colsInRoom
			row := i / colsInRoom
			dstX = col * tileW
			dstY = row * tileH
		}

		srcRect := image.Rect(srcX, srcY, srcX+tileW, srcY+tileH)
		dstRect := image.Rect(dstX, dstY, dstX+tileW, dstY+tileH)

		draw.Draw(dst, dstRect, tilesetImg, srcRect.Min, draw.Over)
	}

	return dst, nil
}

// ProcessObjectSpriteSheet reads the sprite sheet image for an object, computes sub_sprites_total_count,
// and populates the object's Sprites array with sub-sprite index entries.
func ProcessObjectSpriteSheet(obj *Object, baseDir string) error {
	if obj.SpriteSheet == "" {
		obj.SubSpritesTotalCount = 0
		obj.Sprites = nil
		return nil
	}

	if obj.SubSpriteWidth <= 0 || obj.SubSpriteHeight <= 0 {
		return fmt.Errorf("invalid sub-sprite dimensions: %dx%d", obj.SubSpriteWidth, obj.SubSpriteHeight)
	}

	spriteSheetPath := obj.SpriteSheet
	if !filepath.IsAbs(spriteSheetPath) && baseDir != "" {
		spriteSheetPath = filepath.Join(baseDir, spriteSheetPath)
	}

	f, err := os.Open(spriteSheetPath)
	if err != nil {
		return fmt.Errorf("sprite sheet asset not found at path %q: %w", obj.SpriteSheet, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to decode sprite sheet image %q: %w", obj.SpriteSheet, err)
	}

	bounds := img.Bounds()
	cols := bounds.Dx() / obj.SubSpriteWidth
	rows := bounds.Dy() / obj.SubSpriteHeight

	totalCount := cols * rows
	obj.SubSpritesTotalCount = totalCount

	sprites := make([]SubSprite, totalCount)
	for i := 0; i < totalCount; i++ {
		sprites[i] = SubSprite{Index: i}
	}
	obj.Sprites = sprites

	return nil
}

// LoadSubSprite extracts a single sub-sprite at the given index from an object's sprite sheet as an image.Image.
func LoadSubSprite(obj *Object, index int, baseDir string) (image.Image, error) {
	if obj.SpriteSheet == "" {
		return nil, fmt.Errorf("object sprite_sheet is empty")
	}

	subW := obj.SubSpriteWidth
	subH := obj.SubSpriteHeight
	if subW <= 0 || subH <= 0 {
		return nil, fmt.Errorf("invalid sub-sprite dimensions: %dx%d", subW, subH)
	}

	spriteSheetPath := obj.SpriteSheet
	if !filepath.IsAbs(spriteSheetPath) && baseDir != "" {
		spriteSheetPath = filepath.Join(baseDir, spriteSheetPath)
	}

	f, err := os.Open(spriteSheetPath)
	if err != nil {
		return nil, fmt.Errorf("sprite sheet asset not found at path %q: %w", obj.SpriteSheet, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode sprite sheet image %q: %w", obj.SpriteSheet, err)
	}

	bounds := img.Bounds()
	cols := bounds.Dx() / subW
	rows := bounds.Dy() / subH
	total := cols * rows

	if index < 0 || index >= total {
		return nil, fmt.Errorf("sub-sprite index %d out of bounds (total: %d)", index, total)
	}

	srcX := (index % cols) * subW
	srcY := (index / cols) * subH

	dst := image.NewRGBA(image.Rect(0, 0, subW, subH))
	draw.Draw(dst, dst.Bounds(), img, image.Pt(bounds.Min.X+srcX, bounds.Min.Y+srcY), draw.Src)

	return dst, nil
}

// LoadSubSprites extracts all sub-sprites from an object's sprite sheet into a slice of image.Image.
func LoadSubSprites(obj *Object, baseDir string) ([]image.Image, error) {
	if obj.SpriteSheet == "" {
		return nil, fmt.Errorf("object sprite_sheet is empty")
	}

	subW := obj.SubSpriteWidth
	subH := obj.SubSpriteHeight
	if subW <= 0 || subH <= 0 {
		return nil, fmt.Errorf("invalid sub-sprite dimensions: %dx%d", subW, subH)
	}

	spriteSheetPath := obj.SpriteSheet
	if !filepath.IsAbs(spriteSheetPath) && baseDir != "" {
		spriteSheetPath = filepath.Join(baseDir, spriteSheetPath)
	}

	f, err := os.Open(spriteSheetPath)
	if err != nil {
		return nil, fmt.Errorf("sprite sheet asset not found at path %q: %w", obj.SpriteSheet, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode sprite sheet image %q: %w", obj.SpriteSheet, err)
	}

	bounds := img.Bounds()
	cols := bounds.Dx() / subW
	rows := bounds.Dy() / subH
	total := cols * rows

	subSprites := make([]image.Image, total)
	for i := 0; i < total; i++ {
		srcX := (i % cols) * subW
		srcY := (i / cols) * subH

		dst := image.NewRGBA(image.Rect(0, 0, subW, subH))
		draw.Draw(dst, dst.Bounds(), img, image.Pt(bounds.Min.X+srcX, bounds.Min.Y+srcY), draw.Src)
		subSprites[i] = dst
	}

	return subSprites, nil
}

// RenderRoomComposite renders all tileset layers in room order into a single composite image.Image.
func RenderRoomComposite(room *Room, baseDir string) (image.Image, error) {
	width := room.Width
	height := room.Height
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	composite := image.NewRGBA(image.Rect(0, 0, width, height))

	for _, layer := range room.TilesetLayers {
		layerImg, err := RenderTilesetLayer(&layer, width, height, baseDir)
		if err != nil {
			return nil, err
		}
		draw.Draw(composite, composite.Bounds(), layerImg, image.Point{}, draw.Over)
	}

	return composite, nil
}
