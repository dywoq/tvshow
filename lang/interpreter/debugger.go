// Package interpreter executes Scintilla bytecode programs.
package interpreter

import (
	"errors"
	"strings"
	"sync"

	"tvshow/lang/bytecode"
	"tvshow/lang/token"
)

// ErrStopped is returned when execution is halted by the debugger.
var ErrStopped = errors.New("debugger stopped execution")

// DebugEventKind specifies the type of debug event that paused execution.
type DebugEventKind int

const (
	EventBreakpoint DebugEventKind = iota
	EventStep
	EventPause
)

func (k DebugEventKind) String() string {
	switch k {
	case EventBreakpoint:
		return "breakpoint"
	case EventStep:
		return "step"
	case EventPause:
		return "pause"
	default:
		return "unknown"
	}
}

// DebugAction determines how execution continues after a debug event.
type DebugAction int

const (
	ActionContinue DebugAction = iota
	ActionStepInto
	ActionStepOver
	ActionStepOut
	ActionStepInstruction
	ActionStop
)

// DebugEvent contains details about the current execution state when paused.
type DebugEvent struct {
	Kind        DebugEventKind
	Position    token.Position
	FuncName    string
	PC          int
	Instruction bytecode.Instruction
}

// HookFunc is called when a debug event occurs. It returns the next DebugAction.
type HookFunc func(dbg *Debugger, event DebugEvent) DebugAction

// Breakpoint represents a source location or function breakpoint.
type Breakpoint struct {
	ID       int
	Filename string
	Line     int
	FuncName string
	Enabled  bool
}

// StackFrame describes a call stack frame.
type StackFrame struct {
	FuncName    string
	Position    token.Position
	PC          int
	Instruction bytecode.Instruction
	Locals      map[string]any
}

// Debugger manages execution state, breakpoints, stepping, and state inspection.
type Debugger struct {
	mu sync.RWMutex

	hook HookFunc

	breakpoints []*Breakpoint
	nextBPID    int

	action         DebugAction
	pauseRequested bool

	stepStartLine  int
	stepStartFile  string
	stepStartDepth int

	interp *Interpreter
}

// NewDebugger creates a new Debugger instance.
func NewDebugger() *Debugger {
	return &Debugger{
		action: ActionContinue,
	}
}

// Attach binds this debugger to an Interpreter instance.
func (dbg *Debugger) Attach(interp *Interpreter) {
	if interp != nil {
		interp.SetDebugger(dbg)
	}
}

// SetHook configures the callback invoked when execution hits a breakpoint, step, or pause.
func (dbg *Debugger) SetHook(fn HookFunc) {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.hook = fn
}

// SetBreakpoint registers a line breakpoint.
func (dbg *Debugger) SetBreakpoint(filename string, line int) *Breakpoint {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()

	dbg.nextBPID++
	bp := &Breakpoint{
		ID:       dbg.nextBPID,
		Filename: filename,
		Line:     line,
		Enabled:  true,
	}
	dbg.breakpoints = append(dbg.breakpoints, bp)
	return bp
}

// SetFunctionBreakpoint registers a breakpoint at the entry of a function.
func (dbg *Debugger) SetFunctionBreakpoint(funcName string) *Breakpoint {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()

	dbg.nextBPID++
	bp := &Breakpoint{
		ID:       dbg.nextBPID,
		FuncName: funcName,
		Enabled:  true,
	}
	dbg.breakpoints = append(dbg.breakpoints, bp)
	return bp
}

// ClearBreakpoint removes a breakpoint by its ID.
func (dbg *Debugger) ClearBreakpoint(id int) bool {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()

	for i, bp := range dbg.breakpoints {
		if bp.ID == id {
			dbg.breakpoints = append(dbg.breakpoints[:i], dbg.breakpoints[i+1:]...)
			return true
		}
	}
	return false
}

// ClearBreakpointAt removes breakpoints at a specific line.
func (dbg *Debugger) ClearBreakpointAt(filename string, line int) bool {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()

	removed := false
	var remaining []*Breakpoint
	for _, bp := range dbg.breakpoints {
		if bp.Line == line && matchFilename(filename, bp.Filename) {
			removed = true
		} else {
			remaining = append(remaining, bp)
		}
	}
	dbg.breakpoints = remaining
	return removed
}

// ClearFunctionBreakpoint removes breakpoints for a specific function name.
func (dbg *Debugger) ClearFunctionBreakpoint(funcName string) bool {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()

	removed := false
	var remaining []*Breakpoint
	for _, bp := range dbg.breakpoints {
		if bp.FuncName == funcName {
			removed = true
		} else {
			remaining = append(remaining, bp)
		}
	}
	dbg.breakpoints = remaining
	return removed
}

// ClearAllBreakpoints removes all breakpoints.
func (dbg *Debugger) ClearAllBreakpoints() {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.breakpoints = nil
}

// Breakpoints returns a copy of all breakpoints.
func (dbg *Debugger) Breakpoints() []*Breakpoint {
	dbg.mu.RLock()
	defer dbg.mu.RUnlock()

	out := make([]*Breakpoint, len(dbg.breakpoints))
	for i, bp := range dbg.breakpoints {
		copied := *bp
		out[i] = &copied
	}
	return out
}

// Continue resumes execution until the next breakpoint, pause, or error.
func (dbg *Debugger) Continue() DebugAction {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.action = ActionContinue
	return ActionContinue
}

// StepInto steps to the next line or into a function call.
func (dbg *Debugger) StepInto() DebugAction {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.action = ActionStepInto
	return ActionStepInto
}

// StepOver steps to the next line in the current function, executing function calls inline.
func (dbg *Debugger) StepOver() DebugAction {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.action = ActionStepOver
	return ActionStepOver
}

// StepOut runs until the current function returns to its caller.
func (dbg *Debugger) StepOut() DebugAction {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.action = ActionStepOut
	return ActionStepOut
}

// StepInstruction steps a single bytecode instruction.
func (dbg *Debugger) StepInstruction() DebugAction {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.action = ActionStepInstruction
	return ActionStepInstruction
}

// Pause signals execution to pause at the next instruction.
func (dbg *Debugger) Pause() {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.pauseRequested = true
}

// Stop terminates execution immediately.
func (dbg *Debugger) Stop() DebugAction {
	dbg.mu.Lock()
	defer dbg.mu.Unlock()
	dbg.action = ActionStop
	return ActionStop
}

// CallStack returns all active stack frames, with index 0 representing the innermost frame.
func (dbg *Debugger) CallStack() []StackFrame {
	if dbg.interp == nil {
		return nil
	}
	dbg.mu.RLock()
	defer dbg.mu.RUnlock()

	n := len(dbg.interp.callStack)
	frames := make([]StackFrame, n)
	for i := 0; i < n; i++ {
		info := dbg.interp.callStack[n-1-i]
		sf := StackFrame{
			FuncName: info.funcName,
			PC:       info.pc,
			Locals:   make(map[string]any),
		}
		if info.pc >= 0 && info.pc < len(info.code) {
			sf.Instruction = info.code[info.pc]
			sf.Position = sf.Instruction.Position
		}
		if info.local != nil && info.local.vars != nil {
			for k, cell := range info.local.vars {
				if cell != nil {
					sf.Locals[k] = cell.value
				}
			}
		}
		frames[i] = sf
	}
	return frames
}

// CurrentFrame returns the current (innermost) call stack frame.
func (dbg *Debugger) CurrentFrame() (StackFrame, bool) {
	frames := dbg.CallStack()
	if len(frames) == 0 {
		return StackFrame{}, false
	}
	return frames[0], true
}

// Locals returns a map of local variables in the current call frame.
func (dbg *Debugger) Locals() map[string]any {
	frame, ok := dbg.CurrentFrame()
	if !ok {
		return nil
	}
	return frame.Locals
}

// Globals returns a map of global variables.
func (dbg *Debugger) Globals() map[string]any {
	if dbg.interp == nil {
		return nil
	}
	dbg.mu.RLock()
	defer dbg.mu.RUnlock()

	globals := make(map[string]any)
	for k, cell := range dbg.interp.globals.vars {
		if cell != nil {
			globals[k] = cell.value
		}
	}
	return globals
}

// OperandStack returns a copy of the current operand stack.
func (dbg *Debugger) OperandStack() []any {
	if dbg.interp == nil {
		return nil
	}
	dbg.mu.RLock()
	defer dbg.mu.RUnlock()

	if len(dbg.interp.callStack) == 0 {
		return nil
	}
	top := dbg.interp.callStack[len(dbg.interp.callStack)-1]
	if top.stack == nil {
		return nil
	}
	stackCopy := make([]any, len(*top.stack))
	copy(stackCopy, *top.stack)
	return stackCopy
}

// CurrentInstruction returns the instruction currently pointed to by PC.
func (dbg *Debugger) CurrentInstruction() (bytecode.Instruction, bool) {
	if dbg.interp == nil {
		return bytecode.Instruction{}, false
	}
	dbg.mu.RLock()
	defer dbg.mu.RUnlock()

	if len(dbg.interp.callStack) == 0 {
		return bytecode.Instruction{}, false
	}
	top := dbg.interp.callStack[len(dbg.interp.callStack)-1]
	if top.pc >= 0 && top.pc < len(top.code) {
		return top.code[top.pc], true
	}
	return bytecode.Instruction{}, false
}

// GetVariable retrieves a variable's value by name from the current frame or globals.
func (dbg *Debugger) GetVariable(name string) (any, bool) {
	if dbg.interp == nil {
		return nil, false
	}
	dbg.mu.RLock()
	defer dbg.mu.RUnlock()

	if len(dbg.interp.callStack) > 0 {
		top := dbg.interp.callStack[len(dbg.interp.callStack)-1]
		if top.local != nil && top.local.vars != nil {
			if cell, ok := top.local.vars[name]; ok && cell != nil {
				return cell.value, true
			}
		}
	}
	if cell, ok := dbg.interp.globals.vars[name]; ok && cell != nil {
		return cell.value, true
	}
	return nil, false
}

// SetVariable updates a variable's value in the current frame or globals.
func (dbg *Debugger) SetVariable(name string, value any) bool {
	if dbg.interp == nil {
		return false
	}
	dbg.mu.Lock()
	defer dbg.mu.Unlock()

	if len(dbg.interp.callStack) > 0 {
		top := dbg.interp.callStack[len(dbg.interp.callStack)-1]
		if top.local != nil && top.local.vars != nil {
			if cell, ok := top.local.vars[name]; ok && cell != nil {
				cell.value = value
				return true
			}
		}
	}
	if cell, ok := dbg.interp.globals.vars[name]; ok && cell != nil {
		cell.value = value
		return true
	}
	return false
}

func (dbg *Debugger) beforeInstruction(interp *Interpreter, frame *callFrameInfo) error {
	dbg.mu.Lock()

	if dbg.action == ActionStop {
		dbg.mu.Unlock()
		return ErrStopped
	}

	ins := frame.code[frame.pc]
	depth := len(interp.callStack)

	bpMatched := false
	for _, bp := range dbg.breakpoints {
		if !bp.Enabled {
			continue
		}
		if bp.FuncName != "" {
			if frame.pc == 0 && bp.FuncName == frame.funcName {
				bpMatched = true
				break
			}
		} else if bp.Line > 0 {
			if ins.Position.Line == bp.Line && matchFilename(ins.Position.Filename, bp.Filename) {
				bpMatched = true
				break
			}
		}
	}

	var hit bool
	var kind DebugEventKind

	if bpMatched {
		hit = true
		kind = EventBreakpoint
	} else if dbg.pauseRequested {
		hit = true
		kind = EventPause
		dbg.pauseRequested = false
	} else {
		switch dbg.action {
		case ActionStepInstruction:
			hit = true
			kind = EventStep
		case ActionStepInto:
			if ins.Position.Line > 0 && (ins.Position.Line != dbg.stepStartLine || ins.Position.Filename != dbg.stepStartFile || depth != dbg.stepStartDepth) {
				hit = true
				kind = EventStep
			}
		case ActionStepOver:
			if ins.Position.Line > 0 && depth <= dbg.stepStartDepth && (ins.Position.Line != dbg.stepStartLine || ins.Position.Filename != dbg.stepStartFile || depth < dbg.stepStartDepth) {
				hit = true
				kind = EventStep
			}
		case ActionStepOut:
			if depth < dbg.stepStartDepth && ins.Position.Line > 0 {
				hit = true
				kind = EventStep
			}
		}
	}

	if !hit {
		dbg.mu.Unlock()
		return nil
	}

	event := DebugEvent{
		Kind:        kind,
		Position:    ins.Position,
		FuncName:    frame.funcName,
		PC:          frame.pc,
		Instruction: ins,
	}

	hook := dbg.hook
	dbg.mu.Unlock()

	var nextAction DebugAction
	if hook != nil {
		nextAction = hook(dbg, event)
	} else {
		nextAction = ActionContinue
	}

	dbg.mu.Lock()
	dbg.action = nextAction
	dbg.stepStartLine = ins.Position.Line
	dbg.stepStartFile = ins.Position.Filename
	dbg.stepStartDepth = depth
	if dbg.action == ActionStop {
		dbg.mu.Unlock()
		return ErrStopped
	}
	dbg.mu.Unlock()

	return nil
}

func matchFilename(posFile, bpFile string) bool {
	if bpFile == "" {
		return true
	}
	return posFile == bpFile || strings.HasSuffix(posFile, bpFile)
}
