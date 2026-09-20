package runtime

import (
	"fmt"
	"sync"
)

// SignalFunction is a runtime signal's function. It is executed 
// when its signal is activated.
type SignalFunction func() error

type signal struct {
	name      string
	functions []SignalFunction
}

var (
	signalMu sync.Mutex
	signals  = map[string]*signal{}
)

// AddSignal stores a new runtime signal into the underlying map.
// Returns an error if the signal with the same name exists.
func AddSignal(name string, functions []SignalFunction) error {
	signalMu.Lock()
	defer signalMu.Unlock()
	if _, ok := signals[name]; ok {
		return fmt.Errorf("runtime signal %q already exists", name)
	}
	signals[name] = &signal{
		name:      name,
		functions: functions,
	}
	return nil
}

// ActivateSignal executes signal's functions. Returns an error if the signal
// does not exist.
func ActivateSignal(name string) error {
	signalMu.Lock()
	defer signalMu.Unlock()
	if _, ok := signals[name]; !ok {
		return fmt.Errorf("runtime signal %q does not exist", name)
	}
	for _, f := range signals[name].functions {
		err := f()
		if err != nil {
			return fmt.Errorf("one of the runtime signal %q's function returned an error: %v", name, err)
		}
	}
	return nil
}
