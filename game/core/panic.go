package core

import (
	"runtime"
	"slices"
	"sync"
)

type PanicFunction struct {
	Address uintptr
	Name    string
	Line    int
	File    string
}

type PanicLocation struct {
	Pc       uintptr
	Function *PanicFunction
}

type PanicContext struct {
	Location *PanicLocation
}

// PanicHandle contains the information of a panic location, such as a source function,
// program counter and message.
type PanicHandle struct {
	Message string
	Context *PanicContext
}

// PanicManager tracks current panic locations.
type PanicManager struct {
	mu      sync.Mutex
	handles []*PanicHandle
}

func NewPanicManager() *PanicManager {
	return &PanicManager{
		handles: make([]*PanicHandle, 0, 16),
	}
}

func NewPanicHandle(message string) *PanicHandle {
	pc, _, _, _ := runtime.Caller(1)
	f := runtime.FuncForPC(pc)
	file, line := f.FileLine(pc)
	h := &PanicHandle{
		Message: message,
		Context: &PanicContext{
			Location: &PanicLocation{
				Pc: pc,
				Function: &PanicFunction{
					Address: f.Entry(),
					Name:    f.Name(),
					Line:    line,
					File:    file,
				},
			},
		},
	}
	return h
}

// Push pushes handle onto the underlying stack.
func (p *PanicManager) Push(handle *PanicHandle) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handles = append(p.handles, handle)
}

// Pop removes the latest element from the underlying stack.
// Returns if its length is 0.
func (p *PanicManager) Pop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.handles) == 0 {
		return
	}
	p.handles = slices.Delete(p.handles, len(p.handles)-1, len(p.handles))
}

// Empty reports whether the underlying stack is empty or not.
func (p *PanicManager) Empty() bool {
	return len(p.handles) == 0
}
