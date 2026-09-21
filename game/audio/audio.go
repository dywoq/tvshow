package audio

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

type Type string

var (
	mu      sync.Mutex
	context *audio.Context
	players = map[string]*audio.Player{}
)

const (
	TypeOgg Type = "ogg"
)

func Initializer() error {
	mu.Lock()
	defer mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("initializing the audio context failed: %v", r)
		}
	}()
	context = audio.NewContext(44100)
	return nil
}

// Initialize converts the file into stream and stores the initialized player
// into the underlying map. Returns an error if the conversion failed,
// t is an unknown audio type or the audio player with playerName already
// exists.
func Initialize(t Type, playerName string, filepath string) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := players[playerName]; ok {
		return fmt.Errorf("the audio player %q already exists", playerName)
	}
	switch t {
	case TypeOgg:
		content, err := os.ReadFile(filepath)
		if err != nil {
			return err
		}
		stream, err := vorbis.DecodeWithSampleRate(context.SampleRate(), bytes.NewReader(content))
		if err != nil {
			return err
		}
		p, err := context.NewPlayer(stream)
		if err != nil {
			return err
		}
		players[playerName] = p
		return nil
	}
	return fmt.Errorf("unknown audio type: %s", string(t))
}

// Close destroys audio player's resources. Returns an error if it couldn't
// find a player.
func Close(playerName string) error {
	mu.Lock()
	defer mu.Unlock()
	p, ok := players[playerName]
	if !ok {
		return fmt.Errorf("failed to find the player %q", playerName)
	}
	p.PauseAndStopReading()
	delete(players, playerName)
	return nil
}

// IsPlaying checks whether the audio player plays.
func IsPlaying(playerName string) bool {
	mu.Lock()
	defer mu.Unlock()
	p, ok := players[playerName]
	if !ok {
		return false
	}
	return p.IsPlaying()
}

// Rewind rewinds audio of the player to the start.
func Rewind(playerName string) error {
	mu.Lock()
	defer mu.Unlock()
	p, ok := players[playerName]
	if !ok {
		return fmt.Errorf("failed to find the player %q", playerName)
	}
	return p.Rewind()
}

// Pause pauses audio of the player.
func Pause(playerName string) error {
	mu.Lock()
	defer mu.Unlock()
	p, ok := players[playerName]
	if !ok {
		return fmt.Errorf("failed to find the player %q", playerName)
	}
	p.Pause()
	return nil
}

// SetVolume sets the volume of the audio player.
func SetVolume(playerName string, volume float64) error {
	mu.Lock()
	defer mu.Unlock()
	p, ok := players[playerName]
	if !ok {
		return fmt.Errorf("failed to find the player %q", playerName)
	}
	p.SetVolume(volume)
	return nil
}

// GetVolume gets the current volume of the audio player. Returns -1
// if the player wasn't found.
func GetVolume(playerName string) float64 {
	mu.Lock()
	defer mu.Unlock()
	p, ok := players[playerName]
	if !ok {
		return -1
	}
	return p.Volume()
}

// Play searches for the audio player with playerName name and plays it.
// Returns an error if it is not found.
func Play(playerName string) error {
	mu.Lock()
	defer mu.Unlock()
	p, ok := players[playerName]
	if !ok {
		return fmt.Errorf("failed to find the player %q", playerName)
	}
	p.Play()
	return nil
}
