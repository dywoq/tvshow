// Package interpreter executes Scintilla bytecode programs.
package interpreter

import (
	"encoding/binary"
	"fmt"
	"math"
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

// Interpreter executes instructions and, when constructed with a Program, can
// call its translated functions. Values are represented as int64, float64,
// string, or Go aggregate values supplied by callers.
type Interpreter struct {
	program     *bytecode.Program
	globals     frame
	functions   map[string]bytecode.Function
	host        map[string]Function
	initialized bool
}

// New creates an interpreter. Supplying a program enables Run and guest calls.
func New(program ...*bytecode.Program) *Interpreter {
	i := &Interpreter{globals: frame{vars: map[string]*cell{}}, functions: map[string]bytecode.Function{}, host: map[string]Function{}}
	if len(program) > 0 && program[0] != nil {
		i.program = program[0]
		for _, f := range program[0].Functions {
			i.functions[f.Name] = f
		}
	}
	return i
}

// RegisterFunction makes a Go function available to call instructions.
func (i *Interpreter) RegisterFunction(name string, fn Function) { i.host[name] = fn }

// Interpret executes an independent instruction sequence.
func Interpret(code []bytecode.Instruction) (any, error) { return New().Execute(code) }

// InterpretBinary decodes and executes one independently encoded instruction per slice.
func InterpretBinary(code [][]byte) (any, error) { return New().ExecuteBinary(code) }

// Execute executes code with a fresh local scope and returns its return value.
func (i *Interpreter) Execute(code []bytecode.Instruction) (any, error) {
	return i.execute(code, nil, nil)
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
			return nil, err
		}
		i.initialized = true
	}
	return i.call(functionRef{name}, args)
}

func (i *Interpreter) execute(code []bytecode.Instruction, args []any, inherited *frame) (any, error) {
	local := &frame{vars: map[string]*cell{}}
	if inherited != nil {
		local = inherited
	}
	stack := []any{}
	pop := func(ins bytecode.Instruction) (any, error) {
		if len(stack) == 0 {
			return nil, Error{ins.Position, "stack underflow"}
		}
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return v, nil
	}
	for pc := 0; pc < len(code); pc++ {
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
			} else {
				return fail("undefined name " + name)
			}
		case bytecode.LoadIndirect:
			v, e := pop(ins)
			if e != nil {
				return nil, e
			}
			a, ok := v.(address)
			if !ok {
				return fail("load_indirect requires an address")
			}
			stack = append(stack, a.get())
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
				return fail("call requires a function")
			}
			if e != nil {
				if err, ok := e.(Error); ok {
					return nil, err
				}
				return fail(e.Error())
			}
			stack = append(stack, v)
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
			var values []any
			if ad, ok := base.(address); ok {
				if ad.get() == nil {
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
	return i.execute(f.Code, args, local)
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
	var z bytecode.Instruction
	if len(data) < 3 {
		return z, fmt.Errorf("interpreter: truncated instruction")
	}
	if data[0] != 1 {
		return z, fmt.Errorf("interpreter: unsupported bytecode version %d", data[0])
	}
	op, ok := decodeOpcode(data[1])
	if !ok {
		return z, fmt.Errorf("interpreter: unknown binary opcode %d", data[1])
	}
	p := 3
	readString := func() (string, error) {
		if len(data)-p < 4 {
			return "", fmt.Errorf("truncated string")
		}
		n := int(binary.BigEndian.Uint32(data[p:]))
		p += 4
		if n < 0 || len(data)-p < n {
			return "", fmt.Errorf("truncated string")
		}
		s := string(data[p : p+n])
		p += n
		return s, nil
	}
	var operand any
	switch data[2] {
	case 0:
	case 1:
		s, e := readString()
		if e != nil {
			return z, fmt.Errorf("interpreter: %v", e)
		}
		operand = s
	case 2:
		if len(data)-p < 8 {
			return z, fmt.Errorf("interpreter: truncated integer operand")
		}
		operand = int(int64(binary.BigEndian.Uint64(data[p:])))
		p += 8
	default:
		return z, fmt.Errorf("interpreter: unknown operand kind %d", data[2])
	}
	filename, e := readString()
	if e != nil {
		return z, fmt.Errorf("interpreter: %v", e)
	}
	if len(data)-p != 24 {
		return z, fmt.Errorf("interpreter: invalid source position")
	}
	pos := token.Position{Filename: filename, Line: int(int64(binary.BigEndian.Uint64(data[p:]))), Column: int(int64(binary.BigEndian.Uint64(data[p+8:]))), Offset: int(int64(binary.BigEndian.Uint64(data[p+16:])))}
	return bytecode.Instruction{Opcode: op, Operand: operand, Position: pos}, nil
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
func decodeOpcode(n byte) (bytecode.Opcode, bool) {
	ops := []bytecode.Opcode{"", bytecode.Declare, bytecode.PushLiteral, bytecode.Load, bytecode.Address, bytecode.AddressIndex, bytecode.AddressMember, bytecode.LoadIndirect, bytecode.StoreIndirect, bytecode.Unary, bytecode.ToBool, bytecode.Binary, bytecode.Dup, bytecode.Rotate, bytecode.Pop, bytecode.Call, bytecode.Jump, bytecode.JumpIfFalse, bytecode.JumpIfTrue, bytecode.Return, bytecode.MakeArray, bytecode.MakeStruct}
	if int(n) >= len(ops) || n == 0 {
		return "", false
	}
	return ops[n], true
}
