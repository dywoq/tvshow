package keyboard

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// MatchStringToKey converts string presentation of keyboard to [ebiten.Key].
func MatchStringToKey(str string) (key ebiten.Key, ok bool) {
	err := key.UnmarshalText([]byte(str))
	return key, err == nil
}

// Pressed converts str to [ebiten.Key] and checks whether it is pressed.
func Pressed(str string) bool {
	k, ok := MatchStringToKey(str)
	if !ok {
		return false
	}
	return ebiten.IsKeyPressed(k)
}

// JustPressed converts str to [ebiten.Key] and checks whether it is pressed in one tick.
func JustPressed(str string) bool {
	k, ok := MatchStringToKey(str)
	if !ok {
		return false
	}
	return inpututil.IsKeyJustPressed(k)
}

// JustReleased converts str to [ebiten.Key] and checks whether it is released in one tick.
func JustReleased(str string) bool {
	k, ok := MatchStringToKey(str)
	if !ok {
		return false
	}
	return inpututil.IsKeyJustReleased(k)
}
