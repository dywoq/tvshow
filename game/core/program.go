package core

import (
	"fmt"
	"os"
	"sync"
	"tvshow/lang/interpreter"
)

var (
	programMu          sync.Mutex
	programFilepath    = "game.scb"
	programInterpreter *interpreter.Interpreter
)

// InitializeProgram loads the compiled program file "game.scb" and initializes the underlying interpreter.
// Notice that game.scb must contain Scintilla bytecode.
func InitializeProgram() error {
	programMu.Lock()
	defer programMu.Unlock()
	programData, err := os.ReadFile(programFilepath)
	if err != nil {
		return fmt.Errorf("loading the program %q failed: %v", programFilepath, err)
	}
	got, err := interpreter.NewFromBinary(programData)
	if err != nil {
		return fmt.Errorf("initializing interpreter failed: %v", err)
	}
	programInterpreter = got
	programInterpreter.RegisterFunction("print", func(v any) {
		fmt.Printf("%v\n", v)
	})
	return nil
}

func ProgramExecutor(params *ExecutorParams) error {
	_, err := programInterpreter.Run("game_frame")
	return err
}
