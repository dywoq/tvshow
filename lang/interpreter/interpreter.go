// Package interpreter executes Scintilla bytecode programs.
package interpreter

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"tvshow/lang/bytecode"
	"tvshow/lang/token"
)

// Error identifies an instruction that could not be executed.
type Error struct {
	Position token.Position
	Message  string
}

func (e Error) Error() string {
	if e.Position == (token.Position{}) {
		return "interpreter: " + e.Message
	}
	return fmt.Sprintf("%s: %s", e.Position, e.Message)
}

// ExceptionInfo provides full exception details passed to the host exception handler or returned as an error.
type ExceptionInfo struct {
	Message   string
	Position  token.Position
	FuncName  string
	CallStack []StackFrame
}

func (e ExceptionInfo) Error() string {
	if e.Position == (token.Position{}) {
		return "exception: " + e.Message
	}
	return fmt.Sprintf("%s: exception: %s", e.Position, e.Message)
}

// ExceptionHandler is a host callback function invoked when an uncaught exception occurs.
type ExceptionHandler func(info ExceptionInfo)

type uncaughtException struct {
	info ExceptionInfo
}

func (u uncaughtException) Error() string {
	return u.info.Error()
}

// Function is a host function callable from Scintilla code.
type Function func(args []any) (any, error)

type cell struct{ value any }
type address struct {
	get func() any
	set func(any)
}

func cellAddress(c *cell) address {
	return address{get: func() any { return c.value }, set: func(v any) { c.value = v }}
}

type functionRef struct{ name string }
type frame struct{ vars map[string]*cell }

type catchHandler struct {
	targetPC   int
	stackDepth int
}

// Interpreter executes instructions and, when constructed with a Program, can
// call its translated functions. Values are represented as int64, float64,
// string, or Go aggregate values supplied by callers.
type callFrameInfo struct {
	funcName string
	code     []bytecode.Instruction
	pc       int
	local    *frame
	stack    *[]any
	handlers []catchHandler
}

type Interpreter struct {
	program          *bytecode.Program
	globals          frame
	functions        map[string]bytecode.Function
	host             map[string]Function
	initialized      bool
	debugger         *Debugger
	callStack        []*callFrameInfo
	exceptionHandler ExceptionHandler
}

// SetExceptionHandler sets the host exception handler for uncaught exceptions.
func (i *Interpreter) SetExceptionHandler(handler ExceptionHandler) {
	i.exceptionHandler = handler
}

func (i *Interpreter) captureCallStack() []StackFrame {
	n := len(i.callStack)
	frames := make([]StackFrame, n)
	for idx := 0; idx < n; idx++ {
		info := i.callStack[n-1-idx]
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
		frames[idx] = sf
	}
	return frames
}

func (i *Interpreter) handleUncaughtError(err error) error {
	if uncaught, ok := err.(uncaughtException); ok {
		if i.exceptionHandler != nil {
			i.exceptionHandler(uncaught.info)
		}
		return uncaught.info
	}
	if exc, ok := err.(ExceptionInfo); ok {
		if i.exceptionHandler != nil {
			i.exceptionHandler(exc)
		}
		return exc
	}
	return err
}

// SetDebugger binds a Debugger to the Interpreter.
func (i *Interpreter) SetDebugger(d *Debugger) {
	i.debugger = d
	if d != nil {
		d.interp = i
	}
}

// Debugger returns the attached Debugger, if any.
func (i *Interpreter) Debugger() *Debugger {
	return i.debugger
}

func isBuiltinFunction(name string) bool {
	return name == "line" || name == "column" || name == "filename"
}

func (i *Interpreter) currentPosition() token.Position {
	if len(i.callStack) > 0 {
		top := i.callStack[len(i.callStack)-1]
		if top.pc >= 0 && top.pc < len(top.code) {
			return top.code[top.pc].Position
		}
	}
	return token.Position{}
}

func (i *Interpreter) registerBuiltins() {
	if i.host == nil {
		i.host = make(map[string]Function)
	}
	i.host["line"] = func(args []any) (any, error) {
		if len(args) != 0 {
			return nil, Error{Position: i.currentPosition(), Message: "line expects 0 arguments"}
		}
		return int64(i.currentPosition().Line), nil
	}
	i.host["column"] = func(args []any) (any, error) {
		if len(args) != 0 {
			return nil, Error{Position: i.currentPosition(), Message: "column expects 0 arguments"}
		}
		return int64(i.currentPosition().Column), nil
	}
	i.host["filename"] = func(args []any) (any, error) {
		if len(args) != 0 {
			return nil, Error{Position: i.currentPosition(), Message: "filename expects 0 arguments"}
		}
		return i.currentPosition().Filename, nil
	}
}

// New creates an interpreter. Supplying a program enables Run and guest calls.
func New(program ...*bytecode.Program) *Interpreter {
	i := &Interpreter{globals: frame{vars: map[string]*cell{}}, functions: map[string]bytecode.Function{}, host: map[string]Function{}}
	i.registerBuiltins()
	if len(program) > 0 && program[0] != nil {
		i.program = program[0]
		for _, f := range program[0].Functions {
			i.functions[f.Name] = f
		}
	}
	return i
}

// NewFromBinary creates an interpreter from a binary encoded bytecode Program.
func NewFromBinary(programData []byte) (*Interpreter, error) {
	prog, err := bytecode.DecodeProgram(programData)
	if err != nil {
		return nil, fmt.Errorf("interpreter: failed to decode program binary: %w", err)
	}
	return New(prog), nil
}

// InterpretProgramBinary decodes a binary program and executes a target function.
func InterpretProgramBinary(programData []byte, funcName string, args ...any) (any, error) {
	interp, err := NewFromBinary(programData)
	if err != nil {
		return nil, err
	}
	return interp.Run(funcName, args...)
}

// RegisterFunction makes a Go function available to call instructions.
// The fn parameter can be a Function (func([]any) (any, error)) or any typed Go function.
func (i *Interpreter) RegisterFunction(name string, fn any) {
	if isBuiltinFunction(name) {
		panic(fmt.Sprintf("interpreter: cannot override built-in function %q", name))
	}
	wrapped, err := WrapFuncWithInterpreter(i, fn)
	if err != nil {
		panic(fmt.Sprintf("interpreter: RegisterFunction %q: %v", name, err))
	}
	i.host[name] = wrapped
}

// Interpret executes an independent instruction sequence.
func Interpret(code []bytecode.Instruction) (any, error) { return New().Execute(code) }

// InterpretBinary decodes and executes one independently encoded instruction per slice.
func InterpretBinary(code [][]byte) (any, error) { return New().ExecuteBinary(code) }

// Execute executes code with a fresh local scope and returns its return value.
func (i *Interpreter) Execute(code []bytecode.Instruction) (any, error) {
	res, err := i.execute(code, nil, nil)
	if err != nil {
		return nil, i.handleUncaughtError(err)
	}
	return res, nil
}

// ExecuteBinary decodes binary instructions before executing them.
func (i *Interpreter) ExecuteBinary(code [][]byte) (any, error) {
	instructions, err := DecodeInstructions(code)
	if err != nil {
		return nil, err
	}
	return i.Execute(instructions)
}

// Run initializes program globals once and calls a named translated function.
func (i *Interpreter) Run(name string, args ...any) (any, error) {
	if i.program != nil && !i.initialized {
		if _, err := i.execute(i.program.Globals, nil, &i.globals); err != nil {
			return nil, i.handleUncaughtError(err)
		}
		i.initialized = true
	}
	res, err := i.call(functionRef{name}, args)
	if err != nil {
		return nil, i.handleUncaughtError(err)
	}
	return res, nil
}

func (i *Interpreter) execute(code []bytecode.Instruction, args []any, inherited *frame) (any, error) {
	return i.executeFunc("<main>", code, args, inherited)
}

func (i *Interpreter) executeFunc(funcName string, code []bytecode.Instruction, args []any, inherited *frame) (any, error) {
	local := &frame{vars: map[string]*cell{}}
	if inherited != nil {
		local = inherited
	}
	stack := []any{}

	frameInfo := &callFrameInfo{
		funcName: funcName,
		code:     code,
		pc:       0,
		local:    local,
		stack:    &stack,
	}
	i.callStack = append(i.callStack, frameInfo)
	defer func() {
		i.callStack = i.callStack[:len(i.callStack)-1]
	}()

	pop := func(ins bytecode.Instruction) (any, error) {
		if len(stack) == 0 {
			return nil, Error{ins.Position, "stack underflow"}
		}
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return v, nil
	}
	for pc := 0; pc < len(code); pc++ {
		frameInfo.pc = pc
		if i.debugger != nil {
			if err := i.debugger.beforeInstruction(i, frameInfo); err != nil {
				return nil, err
			}
		}
		ins := code[pc]
		fail := func(s string) (any, error) { return nil, Error{ins.Position, s} }
		switch ins.Opcode {
		case bytecode.Declare:
			name, ok := ins.Operand.(string)
			if !ok {
				return fail("declare requires a variable name")
			}
			local.vars[name] = &cell{}
		case bytecode.PushLiteral:
			s, ok := ins.Operand.(string)
			if !ok {
				return fail("push_literal requires a literal")
			}
			v, err := parseLiteral(s)
			if err != nil {
				return fail(err.Error())
			}
			stack = append(stack, v)
		case bytecode.Load:
			name, ok := ins.Operand.(string)
			if !ok {
				return fail("load requires a variable name")
			}
			if c, ok := local.vars[name]; ok && c.value != nil {
				stack = append(stack, c.value)
			} else if c, ok := i.globals.vars[name]; ok && c.value != nil {
				stack = append(stack, c.value)
			} else if _, ok := i.functions[name]; ok {
				stack = append(stack, functionRef{name})
			} else if _, ok := i.host[name]; ok {
				stack = append(stack, functionRef{name})
			} else if c, ok := local.vars[name]; ok {
				stack = append(stack, c.value)
			} else if c, ok := i.globals.vars[name]; ok {
				stack = append(stack, c.value)
			} else {
				return fail("undefined name " + name)
			}
		case bytecode.Address:
			name, ok := ins.Operand.(string)
			if !ok {
				return fail("address requires a variable name")
			}
			if c, ok := local.vars[name]; ok {
				stack = append(stack, cellAddress(c))
			} else if c, ok := i.globals.vars[name]; ok {
				stack = append(stack, cellAddress(c))
			} else if _, ok := i.functions[name]; ok {
				stack = append(stack, functionRef{name})
			} else if _, ok := i.host[name]; ok {
				stack = append(stack, functionRef{name})
			} else {
				return fail("undefined name " + name)
			}
		case bytecode.LoadIndirect:
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			if a, ok := v.(address); ok {
				stack = append(stack, a.get())
			} else if _, ok := v.(functionRef); ok {
				stack = append(stack, v)
			} else if _, ok := v.(Function); ok {
				stack = append(stack, v)
			} else if _, ok := v.(func([]any) (any, error)); ok {
				stack = append(stack, v)
			} else {
				return fail("load_indirect requires an address")
			}
		case bytecode.StoreIndirect:
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			a, e := pop(ins)
			if e != nil {
				return nil, e
			}
			ad, ok := a.(address)
			if !ok {
				return fail("store_indirect requires an address")
			}
			ad.set(v)
			stack = append(stack, v)
		case bytecode.Dup:
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			stack = append(stack, v, v)
		case bytecode.Pop:
			if _, e := pop(ins); e != nil {
				return nil, e
			}
		case bytecode.Rotate:
			if len(stack) < 3 {
				return fail("rotate requires three values")
			}
			n := len(stack)
			stack[n-3], stack[n-2] = stack[n-2], stack[n-3]
		case bytecode.Swap:
			if len(stack) < 2 {
				return fail("swap requires two values on stack")
			}
			n := len(stack)
			stack[n-1], stack[n-2] = stack[n-2], stack[n-1]
		case bytecode.ToBool:
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			stack = append(stack, boolInt(truth(v)))
		case bytecode.Unary:
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			r, e := unary(ins.Operand, v)
			if e != nil {
				return fail(e.Error())
			}
			stack = append(stack, r)
		case bytecode.Binary:
			r, e := pop(ins)
			if e != nil {
				return nil, e
			}
			l, e := pop(ins)
			if e != nil {
				return nil, e
			}
			v, e := operation(ins.Operand, l, r)
			if e != nil {
				return fail(e.Error())
			}
			stack = append(stack, v)
		case bytecode.Call:
			n, ok := ins.Operand.(int)
			if !ok || n < 0 {
				return fail("call requires a non-negative argument count")
			}
			if len(stack) < n+1 {
				return fail("call stack underflow")
			}
			start := len(stack) - n
			argv := append([]any(nil), stack[start:]...)
			target := stack[start-1]
			stack = stack[:start-1]
			var v any
			var e error
			switch fn := target.(type) {
			case functionRef:
				v, e = i.call(fn, argv)
			case Function:
				v, e = fn(argv)
			case func([]any) (any, error):
				v, e = fn(argv)
			case string:
				v, e = i.call(functionRef{fn}, argv)
			case bytecode.Function:
				v, e = i.call(functionRef{fn.Name}, argv)
			default:
				if target != nil && reflect.ValueOf(target).Kind() == reflect.Func {
					if wf, err := WrapFuncWithInterpreter(i, target); err == nil {
						v, e = wf(argv)
						break
					}
				}
				return fail("call requires a function")
			}
			if e != nil {
				if len(frameInfo.handlers) > 0 {
					var excMsg string
					if uncaught, ok := e.(uncaughtException); ok {
						excMsg = uncaught.info.Message
					} else if exc, ok := e.(ExceptionInfo); ok {
						excMsg = exc.Message
					} else {
						excMsg = e.Error()
					}
					h := frameInfo.handlers[len(frameInfo.handlers)-1]
					frameInfo.handlers = frameInfo.handlers[:len(frameInfo.handlers)-1]
					stack = stack[:h.stackDepth]
					stack = append(stack, excMsg)
					pc = h.targetPC - 1
					continue
				}
				if uncaught, ok := e.(uncaughtException); ok {
					return nil, uncaught
				}
				if exc, ok := e.(ExceptionInfo); ok {
					return nil, uncaughtException{info: exc}
				}
				if err, ok := e.(Error); ok {
					return nil, err
				}
				return fail(e.Error())
			}
			stack = append(stack, v)
		case bytecode.PushCatch:
			target, ok := ins.Operand.(int)
			if !ok || target < 0 || target >= len(code) {
				return fail("invalid catch target")
			}
			frameInfo.handlers = append(frameInfo.handlers, catchHandler{
				targetPC:   target,
				stackDepth: len(stack),
			})
		case bytecode.PopCatch:
			if len(frameInfo.handlers) > 0 {
				frameInfo.handlers = frameInfo.handlers[:len(frameInfo.handlers)-1]
			}
		case bytecode.Throw:
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			msg, ok := v.(string)
			if !ok {
				return fail("throw requires a string exception")
			}
			excInfo := ExceptionInfo{
				Message:   msg,
				Position:  ins.Position,
				FuncName:  funcName,
				CallStack: i.captureCallStack(),
			}
			if len(frameInfo.handlers) > 0 {
				h := frameInfo.handlers[len(frameInfo.handlers)-1]
				frameInfo.handlers = frameInfo.handlers[:len(frameInfo.handlers)-1]
				stack = stack[:h.stackDepth]
				stack = append(stack, excInfo.Message)
				pc = h.targetPC - 1
				continue
			}
			return nil, uncaughtException{info: excInfo}
		case bytecode.Jump, bytecode.JumpIfFalse, bytecode.JumpIfTrue:
			target, ok := ins.Operand.(int)
			if !ok || target < 0 || target >= len(code) {
				return fail("invalid jump target")
			}
			take := ins.Opcode == bytecode.Jump
			if !take {
				v, e := pop(ins)
				if e != nil {
					return nil, e
				}
				take = truth(v) == (ins.Opcode == bytecode.JumpIfTrue)
			}
			if take {
				pc = target - 1
			}
		case bytecode.Return:
			if len(stack) == 0 {
				return nil, nil
			}
			return stack[len(stack)-1], nil
		case bytecode.MakeArray:
			n, ok := ins.Operand.(int)
			if !ok || n < 0 {
				return fail("make_array requires a non-negative length")
			}
			stack = append(stack, make([]any, n))
		case bytecode.MakeStruct:
			stack = append(stack, make(map[string]any))
		case bytecode.AddressIndex:
			index, e := pop(ins)
			if e != nil {
				return nil, e
			}
			base, e := pop(ins)
			if e != nil {
				return nil, e
			}
			n, e := integer(index)
			if e != nil || n < 0 {
				return fail("address_index requires a non-negative integer index")
			}
			if ad, ok := base.(address); ok {
				if str, ok := ad.get().(string); ok {
					at := int(n)
					if at < 0 || at >= len(str) {
						return fail("string index out of range")
					}
					stack = append(stack, address{
						get: func() any {
							s := ad.get().(string)
							return int64(s[at])
						},
						set: func(v any) {
							ch, ok := v.(int64)
							if !ok {
								return
							}
							b := []byte(ad.get().(string))
							if at >= 0 && at < len(b) {
								b[at] = byte(ch)
								ad.set(string(b))
							}
						},
					})
					continue
				}
			} else if str, ok := base.(string); ok {
				at := int(n)
				if at < 0 || at >= len(str) {
					return fail("string index out of range")
				}
				stack = append(stack, address{
					get: func() any { return int64(str[at]) },
					set: func(v any) {},
				})
				continue
			}
			var values []any
			if ad, ok := base.(address); ok {
				if ad.get() == nil || isZero(ad.get()) {
					ad.set(make([]any, int(n)+1))
				}
				v, ok := ad.get().([]any)
				if !ok {
					return fail("address_index requires an array aggregate")
				}
				if int(n) >= len(v) {
					newSlice := make([]any, int(n)+1)
					copy(newSlice, v)
					ad.set(newSlice)
					v = newSlice
				}
				values = v
			} else if v, ok := base.([]any); ok {
				if int(n) >= len(v) {
					return fail("array index is out of range")
				}
				values = v
			} else {
				return fail("address_index requires an array aggregate")
			}
			at := int(n)
			stack = append(stack, address{get: func() any { return values[at] }, set: func(v any) { values[at] = v }})
		case bytecode.AddressMember:
			member, ok := ins.Operand.(string)
			if !ok {
				return fail("address_member requires a member name")
			}
			base, e := pop(ins)
			if e != nil {
				return nil, e
			}
			var members map[string]any
			if ad, ok := base.(address); ok {
				if ad.get() == nil {
					ad.set(make(map[string]any))
				}
				m, ok := ad.get().(map[string]any)
				if !ok {
					return fail("address_member requires a map[string]any aggregate")
				}
				members = m
			} else if m, ok := base.(map[string]any); ok {
				members = m
			} else {
				return fail("address_member requires an aggregate address")
			}
			stack = append(stack, address{get: func() any { return members[member] }, set: func(v any) { members[member] = v }})
		case bytecode.Convert:
			targetType, ok := ins.Operand.(string)
			if !ok {
				return fail("convert requires a target type operand")
			}
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			res, e := convertValueType(v, targetType)
			if e != nil {
				return fail(e.Error())
			}
			stack = append(stack, res)
		default:
			return fail("unknown opcode " + string(ins.Opcode))
		}
	}
	return nil, nil
}

func (i *Interpreter) call(ref functionRef, args []any) (any, error) {
	if fn, ok := i.host[ref.name]; ok {
		return fn(args)
	}
	f, ok := i.functions[ref.name]
	if !ok {
		return nil, Error{Message: "undefined function " + ref.name}
	}
	if len(args) != len(f.Parameters) {
		return nil, Error{Message: fmt.Sprintf("%s expects %d arguments, got %d", ref.name, len(f.Parameters), len(args))}
	}
	local := &frame{vars: map[string]*cell{}}
	for n, v := range args {
		local.vars[f.Parameters[n]] = &cell{v}
	}
	return i.executeFunc(ref.name, f.Code, args, local)
}

// ToFunction converts a function reference, function name, or Function into an executable Function.
func (i *Interpreter) ToFunction(v any) (Function, error) {
	if v == nil {
		return nil, fmt.Errorf("interpreter: cannot convert nil to function")
	}
	switch f := v.(type) {
	case Function:
		return f, nil
	case func([]any) (any, error):
		return Function(f), nil
	case functionRef:
		return func(args []any) (any, error) {
			return i.call(f, args)
		}, nil
	case string:
		return func(args []any) (any, error) {
			return i.call(functionRef{f}, args)
		}, nil
	case bytecode.Function:
		return func(args []any) (any, error) {
			return i.call(functionRef{f.Name}, args)
		}, nil
	default:
		if reflect.ValueOf(v).Kind() == reflect.Func {
			return WrapFuncWithInterpreter(i, v)
		}
		return nil, fmt.Errorf("interpreter: %v is not a function", v)
	}
}

// CallFunc invokes a function reference, function name, or Function with arguments.
func (i *Interpreter) CallFunc(fn any, args ...any) (any, error) {
	f, err := i.ToFunction(fn)
	if err != nil {
		return nil, err
	}
	return f(args)
}

func isZero(v any) bool {
	if v == nil {
		return true
	}
	if n, err := integer(v); err == nil {
		return n == 0
	}
	return false
}

func boolInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
func truth(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int64:
		return x != 0
	case int:
		return x != 0
	case int32:
		return x != 0
	case float64:
		return x != 0
	case float32:
		return x != 0
	case string:
		return x != ""
	default:
		return true
	}
}
func parseLiteral(s string) (any, error) {
	if strings.HasPrefix(s, "\"") {
		return strconv.Unquote(s)
	}
	if strings.HasPrefix(s, "'") {
		v, e := strconv.Unquote(s)
		if e != nil {
			return nil, e
		}
		r := []rune(v)
		if len(r) != 1 {
			return nil, fmt.Errorf("invalid character literal %q", s)
		}
		return int64(r[0]), nil
	}
	// First try the spelling unchanged: hexadecimal literals may legitimately
	// end in A through F, which are not suffixes.
	if strings.ContainsAny(s, ".eEpP") {
		if v, e := strconv.ParseFloat(s, 64); e == nil {
			return v, nil
		}
		return strconv.ParseFloat(strings.TrimRight(s, "fFlL"), 64)
	}
	if v, e := strconv.ParseInt(s, 0, 64); e == nil {
		return v, nil
	}
	return strconv.ParseInt(strings.TrimRight(s, "uUlL"), 0, 64)
}
func unary(op any, v any) (any, error) {
	s, ok := op.(string)
	if !ok {
		return nil, fmt.Errorf("unary requires an operator")
	}
	if s == "sizeof" {
		if ad, ok := v.(address); ok {
			v = ad.get()
		}
		if str, ok := v.(string); ok {
			return int64(len(str)), nil
		}
		if arr, ok := v.([]any); ok {
			return int64(len(arr)), nil
		}
		if m, ok := v.(map[string]any); ok {
			return int64(len(m)), nil
		}
		if v == nil {
			return int64(0), nil
		}
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			return int64(val.Len()), nil
		}
		return int64(8), nil
	}
	if s == "stringify" {
		if ad, ok := v.(address); ok {
			v = ad.get()
		}
		return fmt.Sprint(v), nil
	}
	if s == "!" {
		return boolInt(!truth(v)), nil
	}
	if s == "~" {
		n, e := integer(v)
		return ^n, e
	}
	if s == "+" {
		return v, nil
	}
	if s == "-" {
		if f, ok := v.(float64); ok {
			return -f, nil
		}
		n, e := integer(v)
		return -n, e
	}
	return nil, fmt.Errorf("unsupported unary operator %q", s)
}
func integer(v any) (int64, error) {
	if v == nil {
		return 0, nil
	}
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int16:
		return int64(x), nil
	case int8:
		return int64(x), nil
	case uint:
		return int64(x), nil
	case uint64:
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case uint16:
		return int64(x), nil
	case uint8:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case float32:
		return int64(x), nil
	case bool:
		return boolInt(x), nil
	default:
		return 0, fmt.Errorf("integer operand required")
	}
}
func operation(op any, l, r any) (any, error) {
	s, ok := op.(string)
	if !ok {
		return nil, fmt.Errorf("binary requires an operator")
	}
	if s == "==" {
		return boolInt(equal(l, r)), nil
	}
	if s == "!=" {
		return boolInt(!equal(l, r)), nil
	}
	if s == "&&" {
		return boolInt(truth(l) && truth(r)), nil
	}
	if s == "||" {
		return boolInt(truth(l) || truth(r)), nil
	}
	ls, leftStr := l.(string)
	rs, rightStr := r.(string)
	if leftStr || rightStr {
		if s == "+" {
			if leftStr && rightStr {
				return ls + rs, nil
			}
			if leftStr {
				if rc, ok := r.(int64); ok {
					return ls + string(rune(rc)), nil
				}
			}
			if rightStr {
				if lc, ok := l.(int64); ok {
					return string(rune(lc)) + rs, nil
				}
			}
			return nil, fmt.Errorf("cannot append %T to string", r)
		}
		if leftStr && rightStr {
			switch s {
			case "==":
				return boolInt(ls == rs), nil
			case "!=":
				return boolInt(ls != rs), nil
			case "<":
				return boolInt(ls < rs), nil
			case "<=":
				return boolInt(ls <= rs), nil
			case ">":
				return boolInt(ls > rs), nil
			case ">=":
				return boolInt(ls >= rs), nil
			}
		}
	}
	_, leftFloat := l.(float64)
	_, rightFloat := r.(float64)
	if leftFloat || rightFloat {
		lf, _ := asFloat(l)
		rf, ok := asFloat(r)
		if !ok {
			return nil, fmt.Errorf("numeric operand required")
		}
		switch s {
		case "+":
			return lf + rf, nil
		case "-":
			return lf - rf, nil
		case "*":
			return lf * rf, nil
		case "/":
			return lf / rf, nil
		case "<":
			return boolInt(lf < rf), nil
		case ">":
			return boolInt(lf > rf), nil
		case "<=":
			return boolInt(lf <= rf), nil
		case ">=":
			return boolInt(lf >= rf), nil
		}
	}
	a, e := integer(l)
	if e != nil {
		return nil, e
	}
	b, e := integer(r)
	if e != nil {
		return nil, e
	}
	switch s {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return a / b, nil
	case "%":
		if b == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return a % b, nil
	case "&":
		return a & b, nil
	case "|":
		return a | b, nil
	case "^":
		return a ^ b, nil
	case "<<":
		return a << uint64(b), nil
	case ">>":
		return a >> uint64(b), nil
	case "<":
		return boolInt(a < b), nil
	case ">":
		return boolInt(a > b), nil
	case "<=":
		return boolInt(a <= b), nil
	case ">=":
		return boolInt(a >= b), nil
	}
	return nil, fmt.Errorf("unsupported binary operator %q", s)
}
func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case nil:
		return 0, true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int16:
		return float64(n), true
	case int8:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint64:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint8:
		return float64(n), true
	}
	return 0, false
}
func equal(a, b any) bool {
	if af, ok := asFloat(a); ok {
		if bf, ok := asFloat(b); ok {
			return math.Abs(af-bf) == 0
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// DecodeInstruction decodes the stable binary form created by Instruction.MarshalBinary.
func DecodeInstruction(data []byte) (bytecode.Instruction, error) {
	return bytecode.DecodeInstruction(data)
}

func DecodeInstructions(data [][]byte) ([]bytecode.Instruction, error) {
	out := make([]bytecode.Instruction, len(data))
	for n, b := range data {
		v, e := DecodeInstruction(b)
		if e != nil {
			return nil, e
		}
		out[n] = v
	}
	return out, nil
}

func valueTypeName(v any) string {
	if v == nil {
		return "nil"
	}
	switch x := v.(type) {
	case string:
		return "string"
	case float64, float32:
		return "float"
	case int64, int, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return "int"
	case bool:
		return "bool"
	case []any:
		return "array"
	case map[string]any:
		return "struct"
	case functionRef, Function, func([]any) (any, error):
		return "function"
	case address:
		return valueTypeName(x.get())
	default:
		return fmt.Sprintf("%T", v)
	}
}

func convertValueType(v any, targetType string) (any, error) {
	if ad, ok := v.(address); ok {
		v = ad.get()
	}
	if targetType == "auto" {
		return v, nil
	}
	if v == nil {
		if targetType == "void" {
			return nil, nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert nil to %s", targetType)
	}

	srcType := valueTypeName(v)

	switch targetType {
	case "int", "short", "long", "signed", "unsigned":
		if srcType == "int" {
			return integer(v)
		}
		if f, ok := asFloat(v); ok && srcType == "float" {
			return int64(f), nil
		}
		if b, ok := v.(bool); ok {
			return boolInt(b), nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to int", srcType)

	case "float", "double":
		if f, ok := asFloat(v); ok {
			if srcType == "float" || srcType == "int" {
				return f, nil
			}
		}
		if b, ok := v.(bool); ok {
			return float64(boolInt(b)), nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to float", srcType)

	case "char":
		if srcType == "int" {
			n, _ := integer(v)
			return n, nil
		}
		if f, ok := asFloat(v); ok && srcType == "float" {
			return int64(f), nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to char", srcType)

	case "string":
		if s, ok := v.(string); ok {
			return s, nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to string", srcType)

	case "bool", "_Bool":
		if srcType == "int" || srcType == "float" || srcType == "bool" {
			return boolInt(truth(v)), nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to bool", srcType)

	case "array":
		if srcType == "array" {
			return v, nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to array", srcType)

	case "struct", "union":
		if srcType == "struct" {
			return v, nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to %s", srcType, targetType)

	case "pointer":
		if srcType == "int" {
			n, _ := integer(v)
			if n == 0 {
				return nil, nil
			}
		}
		if _, ok := v.(address); ok {
			return v, nil
		}
		if srcType == "array" || srcType == "struct" || srcType == "function" {
			return v, nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to pointer", srcType)

	case "void":
		return nil, nil

	default:
		if srcType == targetType || srcType == "int" || srcType == "struct" {
			return v, nil
		}
		return nil, fmt.Errorf("type conversion error: cannot convert %s to %s", srcType, targetType)
	}
}
