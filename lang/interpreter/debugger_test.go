package interpreter

import (
	"testing"
)

func TestDebuggerBreakpointsAndStepping(t *testing.T) {
	program := programFromSource(t, `
		int add(int a, int b) {
			int sum = a + b;
			return sum;
		}
		int Start() {
			int x = 10;
			int y = 20;
			int z = add(x, y);
			return z;
		}
	`)

	interp := New(program)
	dbg := NewDebugger()
	dbg.Attach(interp)

	var hitEvents []DebugEvent
	dbg.SetHook(func(d *Debugger, event DebugEvent) DebugAction {
		hitEvents = append(hitEvents, event)
		switch len(hitEvents) {
		case 1:
			// Hit breakpoint at Start entry / line. Next, step into.
			if event.Kind != EventBreakpoint {
				t.Errorf("event 1 kind = %v, want EventBreakpoint", event.Kind)
			}
			return ActionStepInto
		case 2:
			// Next instruction/line, step over.
			return ActionStepOver
		case 3:
			// Next line in Start, step into function call add.
			return ActionStepInto
		case 4:
			// Inside add, step out.
			if event.FuncName != "add" {
				t.Errorf("event 4 FuncName = %q, want 'add'", event.FuncName)
			}
			return ActionStepOut
		default:
			return ActionContinue
		}
	})

	// Set breakpoint on function Start
	bp := dbg.SetFunctionBreakpoint("Start")
	if bp == nil || bp.ID == 0 {
		t.Fatalf("SetFunctionBreakpoint failed")
	}

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(30) {
		t.Errorf("Run = %#v, want 30", got)
	}

	if len(hitEvents) < 4 {
		t.Errorf("hitEvents length = %d, want >= 4", len(hitEvents))
	}
}

func TestDebuggerStateInspectionAndVariableModification(t *testing.T) {
	program := programFromSource(t, `
		int global_val = 100;
		int Start(int arg) {
			int local_val = arg + global_val;
			return local_val;
		}
	`)

	interp := New(program)
	dbg := NewDebugger()
	dbg.Attach(interp)

	bp := dbg.SetFunctionBreakpoint("Start")

	dbg.SetHook(func(d *Debugger, event DebugEvent) DebugAction {
		// Check CallStack
		stack := d.CallStack()
		if len(stack) == 0 {
			t.Errorf("CallStack is empty")
		} else {
			if stack[0].FuncName != "Start" {
				t.Errorf("innermost frame func = %q, want 'Start'", stack[0].FuncName)
			}
		}

		// Check GetVariable and SetVariable
		v, ok := d.GetVariable("arg")
		if !ok || v != int64(5) {
			t.Errorf("GetVariable('arg') = (%v, %v), want (5, true)", v, ok)
		}

		gv, ok := d.GetVariable("global_val")
		if !ok || gv != int64(100) {
			t.Errorf("GetVariable('global_val') = (%v, %v), want (100, true)", gv, ok)
		}

		// Modify variable value from debugger hook
		if !d.SetVariable("arg", int64(10)) {
			t.Errorf("SetVariable('arg') failed")
		}
		if !d.SetVariable("global_val", int64(200)) {
			t.Errorf("SetVariable('global_val') failed")
		}

		// Check OperandStack and CurrentInstruction
		_ = d.OperandStack()
		_, ok = d.CurrentInstruction()
		if !ok {
			t.Errorf("CurrentInstruction failed")
		}

		return ActionContinue
	})

	got, err := interp.Run("Start", int64(5))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Because arg was modified to 10 and global_val to 200, output should be 210
	if got != int64(210) {
		t.Errorf("Run = %#v, want 210", got)
	}

	// Verify breakpoint management
	if !dbg.ClearBreakpoint(bp.ID) {
		t.Errorf("ClearBreakpoint failed")
	}
}

func TestDebuggerStepInstructionAndStop(t *testing.T) {
	program := programFromSource(t, `
		int Start() {
			int a = 1;
			int b = 2;
			int c = 3;
			return a + b + c;
		}
	`)

	interp := New(program)
	dbg := NewDebugger()
	dbg.Attach(interp)

	dbg.SetFunctionBreakpoint("Start")

	stepCount := 0
	dbg.SetHook(func(d *Debugger, event DebugEvent) DebugAction {
		stepCount++
		if stepCount == 1 {
			return ActionStepInstruction
		}
		if stepCount == 2 {
			return ActionStepInstruction
		}
		if stepCount == 3 {
			return ActionStop
		}
		return ActionContinue
	})

	_, err := interp.Run("Start")
	if err != ErrStopped {
		t.Errorf("Run error = %v, want ErrStopped", err)
	}
	if stepCount != 3 {
		t.Errorf("stepCount = %d, want 3", stepCount)
	}
}

func TestDebuggerClearBreakpoints(t *testing.T) {
	dbg := NewDebugger()
	bp1 := dbg.SetBreakpoint("main.sc", 10)
	bp2 := dbg.SetBreakpoint("main.sc", 20)
	fbp := dbg.SetFunctionBreakpoint("Foo")

	bps := dbg.Breakpoints()
	if len(bps) != 3 {
		t.Fatalf("len(Breakpoints) = %d, want 3", len(bps))
	}

	if !dbg.ClearBreakpointAt("main.sc", 10) {
		t.Errorf("ClearBreakpointAt failed")
	}
	if len(dbg.Breakpoints()) != 2 {
		t.Errorf("len(Breakpoints) = %d, want 2", len(dbg.Breakpoints()))
	}

	if !dbg.ClearFunctionBreakpoint("Foo") {
		t.Errorf("ClearFunctionBreakpoint failed")
	}
	if len(dbg.Breakpoints()) != 1 {
		t.Errorf("len(Breakpoints) = %d, want 1", len(dbg.Breakpoints()))
	}

	dbg.ClearAllBreakpoints()
	if len(dbg.Breakpoints()) != 0 {
		t.Errorf("len(Breakpoints) = %d, want 0", len(dbg.Breakpoints()))
	}

	_ = bp1
	_ = bp2
	_ = fbp
}

func TestDebuggerPause(t *testing.T) {
	program := programFromSource(t, `
		int Start() {
			int sum = 0;
			for (int i = 0; i < 100; i++) {
				sum += i;
			}
			return sum;
		}
	`)

	interp := New(program)
	dbg := NewDebugger()
	dbg.Attach(interp)

	pauseHit := false
	dbg.SetHook(func(d *Debugger, event DebugEvent) DebugAction {
		if event.Kind == EventPause {
			pauseHit = true
		}
		return ActionContinue
	})

	dbg.Pause()

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(4950) {
		t.Errorf("Run = %#v, want 4950", got)
	}
	if !pauseHit {
		t.Errorf("pauseHit = false, want true")
	}
}

func TestDebuggerGetLocalsAndGlobals(t *testing.T) {
	program := programFromSource(t, `
		int g = 42;
		int Start() {
			int a = 10;
			int b = 20;
			return a + b;
		}
	`)

	interp := New(program)
	dbg := NewDebugger()
	dbg.Attach(interp)

	dbg.SetFunctionBreakpoint("Start")

	dbg.SetHook(func(d *Debugger, event DebugEvent) DebugAction {
		globals := d.Globals()
		if globals["g"] != int64(42) {
			t.Errorf("Globals['g'] = %v, want 42", globals["g"])
		}

		locals := d.Locals()
		if locals == nil {
			t.Errorf("Locals() is nil")
		}

		return ActionContinue
	})

	_, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestDebugEventKindString(t *testing.T) {
	tests := []struct {
		kind DebugEventKind
		want string
	}{
		{EventBreakpoint, "breakpoint"},
		{EventStep, "step"},
		{EventPause, "pause"},
		{DebugEventKind(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("%v.String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestDebuggerUnattachedAndDefaultMethods(t *testing.T) {
	dbg := NewDebugger()
	if stack := dbg.CallStack(); stack != nil {
		t.Errorf("CallStack = %v, want nil", stack)
	}
	if _, ok := dbg.CurrentFrame(); ok {
		t.Errorf("CurrentFrame returned true")
	}
	if dbg.Locals() != nil {
		t.Errorf("Locals = %v, want nil", dbg.Locals())
	}
	if dbg.Globals() != nil {
		t.Errorf("Globals = %v, want nil", dbg.Globals())
	}
	if dbg.OperandStack() != nil {
		t.Errorf("OperandStack = %v, want nil", dbg.OperandStack())
	}
	if _, ok := dbg.CurrentInstruction(); ok {
		t.Errorf("CurrentInstruction returned true")
	}
	if _, ok := dbg.GetVariable("x"); ok {
		t.Errorf("GetVariable returned true")
	}
	if dbg.SetVariable("x", 1) {
		t.Errorf("SetVariable returned true")
	}
}
