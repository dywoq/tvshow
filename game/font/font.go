package font

import (
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	mu            sync.Mutex
	faceSourceMap = map[string]*text.GoTextFaceSource{}
)

// GetFaceSource opens the provided file and returns a [text.GoTextFaceSource]
// struct instance. It stores the instance into cache. Returns an error if it failed
// to open the file or construct the instance.
func GetFaceSource(filepath string) (*text.GoTextFaceSource, error) {
	mu.Lock()
	defer mu.Unlock()
	if val, ok := faceSourceMap[filepath]; ok {
		return val, nil
	}
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	faceSource, err := text.NewGoTextFaceSource(f)
	if err != nil {
		return nil, err
	}
	faceSourceMap[filepath] = faceSource
	return faceSource, nil
}

// ClearCache clears the underlying cache. Subsequent [GetFaceSource] function calls
// have to do heavy operations to load face sources.
func ClearCache() {
	mu.Lock()
	defer mu.Unlock()
	faceSourceMap = map[string]*text.GoTextFaceSource{}
}
