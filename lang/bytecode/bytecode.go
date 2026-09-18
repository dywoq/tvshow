// Package bytecode translates the public parser AST into Scintilla's
// stack-machine instruction stream.  It intentionally does not execute code;
// an interpreter can consume Program without depending on the parser.
package bytecode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"tvshow/lang/parser"
	"tvshow/lang/semantic"
	"tvshow/lang/token"
)

// Opcode identifies an instruction understood by a Scintilla bytecode
// interpreter.  Instructions use a stack for expression values.
type Opcode string

const (
	Declare       Opcode = "declare"
	PushLiteral   Opcode = "push_literal"
	Load          Opcode = "load"
	Address       Opcode = "address"
	AddressIndex  Opcode = "address_index"
	AddressMember Opcode = "address_member"
	LoadIndirect  Opcode = "load_indirect"
	StoreIndirect Opcode = "store_indirect"
	Unary         Opcode = "unary"
	ToBool        Opcode = "to_bool"
	Binary        Opcode = "binary"
	Dup           Opcode = "dup"
	Rotate        Opcode = "rotate"
	Pop           Opcode = "pop"
	Call          Opcode = "call"
	Jump          Opcode = "jump"
	JumpIfFalse   Opcode = "jump_if_false"
	JumpIfTrue    Opcode = "jump_if_true"
	Return        Opcode = "return"
)

// Instruction is one bytecode operation. Operand is a string for names and
// operators, an int for jump targets and call arity, and nil when unused.
// Position preserves source-level debugging information.
type Instruction struct {
	Opcode   Opcode
	Operand  any
	Position token.Position
}

const binaryInstructionVersion byte = 1

const (
	binaryOperandNone byte = iota
	binaryOperandString
	binaryOperandInt
)

// MarshalBinary returns a stable, architecture-independent binary view of an
// instruction. The layout is: version, opcode, operand kind and operand, then
// source position (filename length and bytes, line, column, offset). Integer
// fields use big-endian encoding; source-position integers and integer operands
// are signed 64-bit values. Strings are prefixed by an unsigned 32-bit length.
//
// This implements encoding.BinaryMarshaler. The version byte allows an
// interpreter to reject incompatible bytecode rather than decoding it as a
// different instruction.
func (i Instruction) MarshalBinary() ([]byte, error) {
	opcode, ok := binaryOpcode(i.Opcode)
	if !ok {
		return nil, fmt.Errorf("bytecode: cannot encode unknown opcode %q", i.Opcode)
	}
	var out bytes.Buffer
	out.WriteByte(binaryInstructionVersion)
	out.WriteByte(opcode)
	if err := writeOperand(&out, i.Operand); err != nil {
		return nil, err
	}
	if err := writeString(&out, i.Position.Filename); err != nil {
		return nil, err
	}
	for _, value := range []int{i.Position.Line, i.Position.Column, i.Position.Offset} {
		if err := binary.Write(&out, binary.BigEndian, int64(value)); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// Bytes is a convenience alias for MarshalBinary.
func (i Instruction) Bytes() ([]byte, error) { return i.MarshalBinary() }

func writeOperand(out *bytes.Buffer, operand any) error {
	switch value := operand.(type) {
	case nil:
		out.WriteByte(binaryOperandNone)
	case string:
		out.WriteByte(binaryOperandString)
		return writeString(out, value)
	case int:
		out.WriteByte(binaryOperandInt)
		return binary.Write(out, binary.BigEndian, int64(value))
	default:
		return fmt.Errorf("bytecode: cannot encode operand of type %T", operand)
	}
	return nil
}

func writeString(out *bytes.Buffer, value string) error {
	if uint64(len(value)) > math.MaxUint32 {
		return fmt.Errorf("bytecode: string operand is too large")
	}
	if err := binary.Write(out, binary.BigEndian, uint32(len(value))); err != nil {
		return err
	}
	_, err := out.WriteString(value)
	return err
}

func binaryOpcode(opcode Opcode) (byte, bool) {
	switch opcode {
	case Declare:
		return 1, true
	case PushLiteral:
		return 2, true
	case Load:
		return 3, true
	case Address:
		return 4, true
	case AddressIndex:
		return 5, true
	case AddressMember:
		return 6, true
	case LoadIndirect:
		return 7, true
	case StoreIndirect:
		return 8, true
	case Unary:
		return 9, true
	case ToBool:
		return 10, true
	case Binary:
		return 11, true
	case Dup:
		return 12, true
	case Rotate:
		return 13, true
	case Pop:
		return 14, true
	case Call:
		return 15, true
	case Jump:
		return 16, true
	case JumpIfFalse:
		return 17, true
	case JumpIfTrue:
		return 18, true
	case Return:
		return 19, true
	default:
		return 0, false
	}
}

// Function is a translated function body. Parameters are supplied by the
// caller as the initial local variables, in declaration order.
type Function struct {
	Name       string
	Parameters []string
	Code       []Instruction
}

// Program is a complete translation unit. Globals runs once before any
// function invocation; Functions retains source declaration order.
type Program struct {
	Globals   []Instruction
	Functions []Function
}

// Error reports an AST construct that cannot be represented by the current
// bytecode instruction set.
type Error struct {
	Pos     token.Position
	Message string
}

func (e Error) Error() string { return fmt.Sprintf("%s: %s", e.Pos, e.Message) }

// Translate validates and translates program. Semantic validation is part of
// translation so a bytecode consumer never receives unresolved identifiers or
// invalid control-flow statements.
func Translate(program *parser.Program) (*Program, error) { return New().Translate(program) }

// Translator owns translation state. Create a new Translator for every
// translation unit.
type Translator struct{ next int }

func New() *Translator { return &Translator{} }

func (t *Translator) Translate(ast *parser.Program) (*Program, error) {
	if ast == nil {
		return nil, Error{Message: "cannot translate a nil program"}
	}
	if err := semantic.Analyze(ast); err != nil {
		return nil, err
	}
	out := &Program{}
	for _, d := range ast.Declarations {
		switch d := d.(type) {
		case *parser.VarDecl:
			c := compiler{t: t}
			if err := c.declaration(d); err != nil {
				return nil, err
			}
			if err := c.resolve(); err != nil {
				return nil, err
			}
			out.Globals = append(out.Globals, c.code...)
		case *parser.FunctionDecl:
			if d.Body == nil {
				continue
			}
			c := compiler{t: t, labels: map[string]int{}}
			params := functionParameters(d.Declarator)
			if err := c.block(d.Body); err != nil {
				return nil, err
			}
			// Falling off a C function is represented as a return without a value.
			c.emit(Return, nil, d.Body.Close.Pos)
			if err := c.resolve(); err != nil {
				return nil, err
			}
			out.Functions = append(out.Functions, Function{Name: d.Declarator.Name.Literal, Parameters: params, Code: c.code})
		}
	}
	return out, nil
}

func functionParameters(d parser.Declarator) (out []string) {
	for _, suffix := range d.Suffixes {
		if f, ok := suffix.(*parser.FunctionSuffix); ok {
			for _, p := range f.Parameters {
				if p.Declarator.Name.Literal != "" {
					out = append(out, p.Declarator.Name.Literal)
				}
			}
		}
	}
	return
}

type compiler struct {
	t                 *Translator
	code              []Instruction
	labels            map[string]int
	patches           []patch
	breaks, continues []string
	cases             map[*parser.CaseStmt]caseLabels
}
type patch struct {
	at    int
	label string
	pos   token.Position
}
type caseLabels struct{ entry, body string }

func (c *compiler) name(prefix string) string {
	c.t.next++
	return fmt.Sprintf("%s.%d", prefix, c.t.next)
}
func (c *compiler) emit(op Opcode, operand any, pos token.Position) {
	c.code = append(c.code, Instruction{op, operand, pos})
}
func (c *compiler) label(name string) { c.labels[name] = len(c.code) }
func (c *compiler) jump(op Opcode, label string, pos token.Position) {
	c.patches = append(c.patches, patch{len(c.code), label, pos})
	c.emit(op, 0, pos)
}
func (c *compiler) resolve() error {
	for _, p := range c.patches {
		target, ok := c.labels[p.label]
		if !ok {
			return Error{p.pos, "internal unresolved label " + p.label}
		}
		c.code[p.at].Operand = target
	}
	return nil
}

func (c *compiler) declaration(d *parser.VarDecl) error {
	if hasSpec(d.Specs, token.TYPEDEF) {
		return nil
	}
	for _, v := range d.Declarators {
		if v.Name.Literal == "" {
			continue
		}
		c.emit(Declare, v.Name.Literal, v.Name.Pos)
		if v.Initializer != nil {
			c.emit(Address, v.Name.Literal, v.Name.Pos)
			if err := c.expr(v.Initializer); err != nil {
				return err
			}
			c.emit(StoreIndirect, nil, v.Name.Pos)
			c.emit(Pop, nil, v.Name.Pos)
		}
	}
	return nil
}
func hasSpec(specs []parser.TypeSpec, wanted token.TokenType) bool {
	for _, s := range specs {
		if s.Token.Type == wanted {
			return true
		}
	}
	return false
}

func (c *compiler) block(b *parser.BlockStmt) error {
	if b == nil {
		return nil
	}
	for _, n := range b.Items {
		switch n := n.(type) {
		case *parser.VarDecl:
			if err := c.declaration(n); err != nil {
				return err
			}
		case parser.Statement:
			if err := c.statement(n); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *compiler) statement(s parser.Statement) error {
	switch s := s.(type) {
	case *parser.BlockStmt:
		return c.block(s)
	case *parser.ExprStmt:
		if s.Expr != nil {
			if err := c.expr(s.Expr); err != nil {
				return err
			}
			c.emit(Pop, nil, s.Position())
		}
	case *parser.IfStmt:
		otherwise, end := c.name("if.else"), c.name("if.end")
		if err := c.expr(s.Condition); err != nil {
			return err
		}
		c.jump(JumpIfFalse, otherwise, s.Condition.Position())
		if err := c.statement(s.Then); err != nil {
			return err
		}
		c.jump(Jump, end, s.Position())
		c.label(otherwise)
		if err := c.statement(s.Else); err != nil {
			return err
		}
		c.label(end)
	case *parser.WhileStmt:
		start, end := c.name("while.start"), c.name("while.end")
		c.label(start)
		if err := c.expr(s.Condition); err != nil {
			return err
		}
		c.jump(JumpIfFalse, end, s.Condition.Position())
		c.breaks = append(c.breaks, end)
		c.continues = append(c.continues, start)
		err := c.statement(s.Body)
		c.breaks = c.breaks[:len(c.breaks)-1]
		c.continues = c.continues[:len(c.continues)-1]
		if err != nil {
			return err
		}
		c.jump(Jump, start, s.Position())
		c.label(end)
	case *parser.DoWhileStmt:
		start, condition, end := c.name("do.start"), c.name("do.condition"), c.name("do.end")
		c.label(start)
		c.breaks = append(c.breaks, end)
		c.continues = append(c.continues, condition)
		err := c.statement(s.Body)
		c.breaks = c.breaks[:len(c.breaks)-1]
		c.continues = c.continues[:len(c.continues)-1]
		if err != nil {
			return err
		}
		c.label(condition)
		if err := c.expr(s.Condition); err != nil {
			return err
		}
		c.jump(JumpIfTrue, start, s.Condition.Position())
		c.label(end)
	case *parser.ForStmt:
		if d, ok := s.Init.(*parser.VarDecl); ok {
			if err := c.declaration(d); err != nil {
				return err
			}
		} else if x, ok := s.Init.(*parser.ExprStmt); ok {
			if err := c.statement(x); err != nil {
				return err
			}
		}
		start, post, end := c.name("for.start"), c.name("for.post"), c.name("for.end")
		c.label(start)
		if s.Condition != nil {
			if err := c.expr(s.Condition); err != nil {
				return err
			}
			c.jump(JumpIfFalse, end, s.Condition.Position())
		}
		c.breaks = append(c.breaks, end)
		c.continues = append(c.continues, post)
		err := c.statement(s.Body)
		c.breaks = c.breaks[:len(c.breaks)-1]
		c.continues = c.continues[:len(c.continues)-1]
		if err != nil {
			return err
		}
		c.label(post)
		if s.Post != nil {
			if err := c.expr(s.Post); err != nil {
				return err
			}
			c.emit(Pop, nil, s.Post.Position())
		}
		c.jump(Jump, start, s.Position())
		c.label(end)
	case *parser.SwitchStmt:
		return c.switchStmt(s)
	case *parser.CaseStmt:
		labels := c.cases[s]
		c.label(labels.body)
		return c.statement(s.Body)
	case *parser.LabelStmt:
		c.label("user." + s.Name.Literal)
		return c.statement(s.Statement)
	case *parser.JumpStmt:
		switch s.Token.Type {
		case token.RETURN:
			if s.Value != nil {
				if err := c.expr(s.Value); err != nil {
					return err
				}
			}
			c.emit(Return, nil, s.Token.Pos)
		case token.BREAK:
			c.jump(Jump, c.breaks[len(c.breaks)-1], s.Token.Pos)
		case token.CONTINUE:
			c.jump(Jump, c.continues[len(c.continues)-1], s.Token.Pos)
		case token.GOTO:
			c.jump(Jump, "user."+s.Label.Literal, s.Token.Pos)
		}
	}
	return nil
}

func (c *compiler) switchStmt(s *parser.SwitchStmt) error {
	cases := collectCases(s.Body)
	end := c.name("switch.end")
	value := c.name("switch.value")
	c.emit(Declare, value, s.Token.Pos)
	c.emit(Address, value, s.Token.Pos)
	if err := c.expr(s.Value); err != nil {
		return err
	}
	c.emit(StoreIndirect, nil, s.Token.Pos)
	c.emit(Pop, nil, s.Token.Pos)
	previousCases := c.cases
	c.cases = make(map[*parser.CaseStmt]caseLabels, len(cases))
	var defaultCase *parser.CaseStmt
	for _, item := range cases {
		labels := caseLabels{entry: c.name("switch.case.entry"), body: c.name("switch.case.body")}
		c.cases[item] = labels
		if item.Value == nil {
			defaultCase = item
			continue
		}
		c.emit(Load, value, s.Token.Pos)
		if err := c.expr(item.Value); err != nil {
			return err
		}
		c.emit(Binary, "==", item.Token.Pos)
		c.jump(JumpIfTrue, labels.entry, item.Token.Pos)
	}
	if defaultCase != nil {
		c.jump(Jump, c.cases[defaultCase].entry, s.Token.Pos)
	} else {
		c.jump(Jump, end, s.Token.Pos)
	}
	for _, item := range cases {
		labels := c.cases[item]
		c.label(labels.entry)
		c.jump(Jump, labels.body, item.Token.Pos)
	}
	c.breaks = append(c.breaks, end)
	err := c.statement(s.Body)
	c.breaks = c.breaks[:len(c.breaks)-1]
	c.cases = previousCases
	if err != nil {
		return err
	}
	c.label(end)
	return nil
}

// collectCases finds labels owned by this switch, but does not enter nested
// switches because their case labels belong to the nested dispatch.
func collectCases(s parser.Statement) (out []*parser.CaseStmt) {
	var visit func(parser.Statement)
	visit = func(n parser.Statement) {
		switch n := n.(type) {
		case *parser.CaseStmt:
			out = append(out, n)
			visit(n.Body)
		case *parser.BlockStmt:
			for _, item := range n.Items {
				if child, ok := item.(parser.Statement); ok {
					visit(child)
				}
			}
		case *parser.LabelStmt:
			visit(n.Statement)
		case *parser.IfStmt:
			visit(n.Then)
			visit(n.Else)
		case *parser.WhileStmt:
			visit(n.Body)
		case *parser.DoWhileStmt:
			visit(n.Body)
		case *parser.ForStmt:
			visit(n.Body)
		case *parser.SwitchStmt: // nested dispatch owns its labels
		}
	}
	visit(s)
	return
}

func (c *compiler) expr(e parser.Expression) error {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *parser.IdentExpr:
		c.emit(Load, e.Token.Literal, e.Token.Pos)
	case *parser.LiteralExpr:
		c.emit(PushLiteral, e.Token.Literal, e.Token.Pos)
	case *parser.UnaryExpr:
		if e.Operator.Type == token.BIT_AND {
			return c.address(e.Operand)
		}
		if e.Operator.Type == token.ASTERISK {
			if err := c.expr(e.Operand); err != nil {
				return err
			}
			c.emit(LoadIndirect, nil, e.Operator.Pos)
			return nil
		}
		if e.Operator.Type == token.INCREMENT || e.Operator.Type == token.DECREMENT {
			return c.increment(e)
		}
		if err := c.expr(e.Operand); err != nil {
			return err
		}
		c.emit(Unary, e.Operator.Literal, e.Operator.Pos)
	case *parser.BinaryExpr:
		if e.Operator.Type == token.LOGICAL_AND || e.Operator.Type == token.LOGICAL_OR {
			return c.logical(e)
		}
		if err := c.expr(e.Left); err != nil {
			return err
		}
		if err := c.expr(e.Right); err != nil {
			return err
		}
		c.emit(Binary, e.Operator.Literal, e.Operator.Pos)
	case *parser.AssignExpr:
		return c.assign(e)
	case *parser.ConditionalExpr:
		otherwise, end := c.name("conditional.else"), c.name("conditional.end")
		if err := c.expr(e.Condition); err != nil {
			return err
		}
		c.jump(JumpIfFalse, otherwise, e.Condition.Position())
		if err := c.expr(e.Then); err != nil {
			return err
		}
		c.jump(Jump, end, e.Question.Pos)
		c.label(otherwise)
		if err := c.expr(e.Else); err != nil {
			return err
		}
		c.label(end)
	case *parser.CallExpr:
		// C evaluates the function designator before its arguments.
		if err := c.expr(e.Function); err != nil {
			return err
		}
		for _, arg := range e.Arguments {
			if err := c.expr(arg); err != nil {
				return err
			}
		}
		c.emit(Call, len(e.Arguments), e.Open.Pos)
	case *parser.IndexExpr, *parser.MemberExpr:
		if err := c.address(e); err != nil {
			return err
		}
		c.emit(LoadIndirect, nil, e.Position())
	case *parser.CommaExpr:
		for i, x := range e.Expressions {
			if err := c.expr(x); err != nil {
				return err
			}
			if i+1 < len(e.Expressions) {
				c.emit(Pop, nil, x.Position())
			}
		}
	default:
		return Error{e.Position(), fmt.Sprintf("%T is not supported by the bytecode translator", e)}
	}
	return nil
}

func (c *compiler) address(e parser.Expression) error {
	switch e := e.(type) {
	case *parser.IdentExpr:
		c.emit(Address, e.Token.Literal, e.Token.Pos)
	case *parser.UnaryExpr:
		if e.Operator.Type != token.ASTERISK {
			return Error{e.Position(), "expression is not addressable"}
		}
		return c.expr(e.Operand)
	case *parser.IndexExpr:
		if err := c.expr(e.Value); err != nil {
			return err
		}
		if err := c.expr(e.Index); err != nil {
			return err
		}
		c.emit(AddressIndex, nil, e.Open.Pos)
	case *parser.MemberExpr:
		if err := c.address(e.Value); err != nil {
			return err
		}
		c.emit(AddressMember, e.Member.Literal, e.Operator.Pos)
	default:
		return Error{e.Position(), "expression is not addressable"}
	}
	return nil
}
func (c *compiler) assign(e *parser.AssignExpr) error {
	if err := c.address(e.Left); err != nil {
		return err
	}
	if e.Operator.Type == token.ASSIGN {
		if err := c.expr(e.Right); err != nil {
			return err
		}
		c.emit(StoreIndirect, nil, e.Operator.Pos)
		return nil
	}
	// The address is retained while its old value and the right operand are
	// evaluated: address, address, old-value, right-value -> address, result.
	c.emit(Dup, nil, e.Operator.Pos)
	c.emit(LoadIndirect, nil, e.Operator.Pos)
	if err := c.expr(e.Right); err != nil {
		return err
	}
	c.emit(Binary, compoundOperator(e.Operator.Type), e.Operator.Pos)
	c.emit(StoreIndirect, nil, e.Operator.Pos)
	return nil
}
func compoundOperator(op token.TokenType) string {
	switch op {
	case token.PLUS_ASSIGN:
		return "+"
	case token.MINUS_ASSIGN:
		return "-"
	case token.ASTERISK_ASSIGN:
		return "*"
	case token.SLASH_ASSIGN:
		return "/"
	case token.PERCENT_ASSIGN:
		return "%"
	case token.BIT_AND_ASSIGN:
		return "&"
	case token.BIT_OR_ASSIGN:
		return "|"
	case token.BIT_XOR_ASSIGN:
		return "^"
	case token.SHL_ASSIGN:
		return "<<"
	case token.SHR_ASSIGN:
		return ">>"
	}
	return ""
}
func (c *compiler) increment(e *parser.UnaryExpr) error {
	if err := c.address(e.Operand); err != nil {
		return err
	}
	c.emit(Dup, nil, e.Operator.Pos)
	c.emit(LoadIndirect, nil, e.Operator.Pos)
	if e.Postfix {
		c.emit(Dup, nil, e.Operator.Pos)
	}
	c.emit(PushLiteral, "1", e.Operator.Pos)
	op := "+"
	if e.Operator.Type == token.DECREMENT {
		op = "-"
	}
	c.emit(Binary, op, e.Operator.Pos)
	if e.Postfix {
		// address, old, new -> old, address, new; retaining old is the
		// expression result while StoreIndirect writes new.
		c.emit(Rotate, nil, e.Operator.Pos)
	}
	c.emit(StoreIndirect, nil, e.Operator.Pos)
	if e.Postfix {
		c.emit(Pop, nil, e.Operator.Pos)
	}
	return nil
}
func (c *compiler) logical(e *parser.BinaryExpr) error {
	end := c.name("logical.end")
	if err := c.expr(e.Left); err != nil {
		return err
	}
	c.emit(ToBool, nil, e.Left.Position())
	if e.Operator.Type == token.LOGICAL_AND {
		c.jump(JumpIfFalse, end, e.Operator.Pos)
	} else {
		c.jump(JumpIfTrue, end, e.Operator.Pos)
	}
	if err := c.expr(e.Right); err != nil {
		return err
	}
	c.emit(ToBool, nil, e.Right.Position())
	c.label(end)
	return nil
}
