package audio

import (
	"fmt"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

var (
	mu      sync.Mutex
	context *audio.Context
	player  *audio.Player
)

func Initializer() error {
	mu.Lock()
	defer mu.Unlock()
	context = audio.NewContext(44100)
	if r := recover(); r != nil {
		return fmt.Errorf("initializing the audio context failed: %v", r)
	}
	return nil
}
