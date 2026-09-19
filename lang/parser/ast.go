// Package parser turns Scintilla's (C99) token stream into an abstract syntax tree.
package parser

import (
	"strings"

	"tvshow/lang/token"
)

// Node is implemented by every AST node. Pos is the first token belonging to it.
type Node interface {
	Position() token.Position
	String() string
}

type Expression interface {
	Node
	expression()
}
type Statement interface {
	Node
	statement()
}
type Declaration interface {
	Node
	declaration()
}

type Program struct {
	Declarations []Declaration
	End          token.Token
}

func (n *Program) Position() token.Position {
	if len(n.Declarations) > 0 {
		return n.Declarations[0].Position()
	}
	return n.End.Pos
}
func (n *Program) String() string { return joinNodes(declarationsToNodes(n.Declarations), "\n") }

type TypeSpec struct {
	Token      token.Token
	Qualifiers []token.Token
	Tag        string
	Members    []Declaration
	EnumValues []Enumerator
}

func (n TypeSpec) Position() token.Position { return n.Token.Pos }

type Enumerator struct {
	Name  token.Token
	Value Expression
}
type Declarator struct {
	Name         token.Token
	Pointers     []Pointer
	FuncPointers []Pointer
	Suffixes     []DeclaratorSuffix
	Initializer  Expression
}
type Pointer struct {
	Token      token.Token
	Qualifiers []token.Token
}
type DeclaratorSuffix interface {
	Node
	declaratorSuffix()
}
type ArraySuffix struct {
	Open       token.Token
	Size       Expression
	Qualifiers []token.Token
	Static     bool
}

func (n *ArraySuffix) Position() token.Position { return n.Open.Pos }
func (*ArraySuffix) declaratorSuffix()          {}
func (n *ArraySuffix) String() string           { return "[" + nodeString(n.Size) + "]" }

type FunctionSuffix struct {
	Open       token.Token
	Parameters []Parameter
	Variadic   bool
}

func (n *FunctionSuffix) Position() token.Position { return n.Open.Pos }
func (*FunctionSuffix) declaratorSuffix()          {}
func (n *FunctionSuffix) String() string           { return "(...)" }

type Parameter struct {
	Specs      []TypeSpec
	Declarator Declarator
}

type VarDecl struct {
	Specs       []TypeSpec
	Declarators []Declarator
	Semi        token.Token
}

func (n *VarDecl) Position() token.Position { return n.Specs[0].Position() }
func (*VarDecl) declaration()               {}
func (n *VarDecl) String() string {
	return typeSpecsString(n.Specs) + " " + declaratorsString(n.Declarators) + ";"
}

type FunctionDecl struct {
	Specs      []TypeSpec
	Declarator Declarator
	Body       *BlockStmt
}

func (n *FunctionDecl) Position() token.Position { return n.Specs[0].Position() }
func (*FunctionDecl) declaration()               {}
func (n *FunctionDecl) String() string {
	return typeSpecsString(n.Specs) + " " + n.Declarator.Name.Literal + " " + nodeString(n.Body)
}

type StaticAssertDecl struct {
	Token     token.Token
	Condition Expression
	Message   token.Token
}

func (n *StaticAssertDecl) Position() token.Position { return n.Token.Pos }
func (*StaticAssertDecl) declaration()               {}
func (n *StaticAssertDecl) String() string {
	return n.Token.Literal + "(" + nodeString(n.Condition) + ")"
}

type BlockStmt struct {
	Open  token.Token
	Items []Node
	Close token.Token
}

func (n *BlockStmt) Position() token.Position { return n.Open.Pos }
func (*BlockStmt) statement()                 {}
func (n *BlockStmt) String() string           { return "{" + joinNodes(n.Items, " ") + "}" }

type ExprStmt struct {
	Expr Expression
	Semi token.Token
}

func (n *ExprStmt) Position() token.Position {
	if n.Expr != nil {
		return n.Expr.Position()
	}
	return n.Semi.Pos
}
func (*ExprStmt) statement()       {}
func (n *ExprStmt) String() string { return nodeString(n.Expr) + ";" }

type IfStmt struct {
	Token      token.Token
	Condition  Expression
	Then, Else Statement
}

func (n *IfStmt) Position() token.Position { return n.Token.Pos }
func (*IfStmt) statement()                 {}
func (n *IfStmt) String() string {
	return "if (" + nodeString(n.Condition) + ") " + nodeString(n.Then) + elseString(n.Else)
}

type SwitchStmt struct {
	Token token.Token
	Value Expression
	Body  Statement
}

func (n *SwitchStmt) Position() token.Position { return n.Token.Pos }
func (*SwitchStmt) statement()                 {}
func (n *SwitchStmt) String() string {
	return "switch (" + nodeString(n.Value) + ") " + nodeString(n.Body)
}

type WhileStmt struct {
	Token     token.Token
	Condition Expression
	Body      Statement
}

func (n *WhileStmt) Position() token.Position { return n.Token.Pos }
func (*WhileStmt) statement()                 {}
func (n *WhileStmt) String() string {
	return "while (" + nodeString(n.Condition) + ") " + nodeString(n.Body)
}

type DoWhileStmt struct {
	Token     token.Token
	Body      Statement
	Condition Expression
}

func (n *DoWhileStmt) Position() token.Position { return n.Token.Pos }
func (*DoWhileStmt) statement()                 {}
func (n *DoWhileStmt) String() string {
	return "do " + nodeString(n.Body) + " while (" + nodeString(n.Condition) + ");"
}

type ForStmt struct {
	Token           token.Token
	Init            Node
	Condition, Post Expression
	Body            Statement
}

func (n *ForStmt) Position() token.Position { return n.Token.Pos }
func (*ForStmt) statement()                 {}
func (n *ForStmt) String() string           { return "for (...) " + nodeString(n.Body) }

type JumpStmt struct {
	Token token.Token
	Label token.Token
	Value Expression
	Semi  token.Token
}

func (n *JumpStmt) Position() token.Position { return n.Token.Pos }
func (*JumpStmt) statement()                 {}
func (n *JumpStmt) String() string {
	if n.Value != nil {
		return n.Token.Literal + " " + n.Value.String() + ";"
	}
	if n.Label.Literal != "" {
		return n.Token.Literal + " " + n.Label.Literal + ";"
	}
	return n.Token.Literal + ";"
}

type ThrowStmt struct {
	Token token.Token
	Value Expression
	Semi  token.Token
}

func (n *ThrowStmt) Position() token.Position { return n.Token.Pos }
func (*ThrowStmt) statement()                 {}
func (n *ThrowStmt) String() string {
	return "throw " + nodeString(n.Value) + ";"
}

type CatchBlock struct {
	Token      token.Token
	VarType    []TypeSpec
	Declarator Declarator
	Body       *BlockStmt
}

func (n *CatchBlock) Position() token.Position { return n.Token.Pos }
func (n *CatchBlock) String() string {
	declStr := typeSpecsString(n.VarType)
	if n.Declarator.Name.Literal != "" {
		declStr += " " + n.Declarator.Name.Literal
	}
	return "catch (" + declStr + ") " + nodeString(n.Body)
}

type TryCatchStmt struct {
	Token token.Token
	Body  *BlockStmt
	Catch *CatchBlock
}

func (n *TryCatchStmt) Position() token.Position { return n.Token.Pos }
func (*TryCatchStmt) statement()                 {}
func (n *TryCatchStmt) String() string {
	if n.Catch != nil {
		return "try " + nodeString(n.Body) + " " + nodeString(n.Catch)
	}
	return "try " + nodeString(n.Body)
}

type LabelStmt struct {
	Name      token.Token
	Colon     token.Token
	Statement Statement
}

func (n *LabelStmt) Position() token.Position { return n.Name.Pos }
func (*LabelStmt) statement()                 {}
func (n *LabelStmt) String() string           { return n.Name.Literal + ": " + nodeString(n.Statement) }

type CaseStmt struct {
	Token token.Token
	Value Expression
	Body  Statement
}

func (n *CaseStmt) Position() token.Position { return n.Token.Pos }
func (*CaseStmt) statement()                 {}
func (n *CaseStmt) String() string {
	if n.Value == nil {
		return "default: " + nodeString(n.Body)
	}
	return "case " + n.Value.String() + ": " + nodeString(n.Body)
}

type IdentExpr struct{ Token token.Token }

func (n *IdentExpr) Position() token.Position { return n.Token.Pos }
func (*IdentExpr) expression()                {}
func (n *IdentExpr) String() string           { return n.Token.Literal }

type LiteralExpr struct{ Token token.Token }

func (n *LiteralExpr) Position() token.Position { return n.Token.Pos }
func (*LiteralExpr) expression()                {}
func (n *LiteralExpr) String() string           { return n.Token.Literal }

type UnaryExpr struct {
	Operator token.Token
	Operand  Expression
	Postfix  bool
}

func (n *UnaryExpr) Position() token.Position { return n.Operator.Pos }
func (*UnaryExpr) expression()                {}
func (n *UnaryExpr) String() string {
	if n.Postfix {
		return nodeString(n.Operand) + n.Operator.Literal
	}
	return n.Operator.Literal + nodeString(n.Operand)
}

type BinaryExpr struct {
	Left     Expression
	Operator token.Token
	Right    Expression
}

func (n *BinaryExpr) Position() token.Position { return n.Left.Position() }
func (*BinaryExpr) expression()                {}
func (n *BinaryExpr) String() string {
	return "(" + nodeString(n.Left) + " " + n.Operator.Literal + " " + nodeString(n.Right) + ")"
}

type ConditionalExpr struct {
	Condition Expression
	Question  token.Token
	Then      Expression
	Else      Expression
}

func (n *ConditionalExpr) Position() token.Position { return n.Condition.Position() }
func (*ConditionalExpr) expression()                {}
func (n *ConditionalExpr) String() string {
	return "(" + nodeString(n.Condition) + " ? " + nodeString(n.Then) + " : " + nodeString(n.Else) + ")"
}

type AssignExpr struct {
	Left     Expression
	Operator token.Token
	Right    Expression
}

func (n *AssignExpr) Position() token.Position { return n.Left.Position() }
func (*AssignExpr) expression()                {}
func (n *AssignExpr) String() string {
	return "(" + nodeString(n.Left) + " " + n.Operator.Literal + " " + nodeString(n.Right) + ")"
}

type CallExpr struct {
	Function  Expression
	Open      token.Token
	Arguments []Expression
}

func (n *CallExpr) Position() token.Position { return n.Function.Position() }
func (*CallExpr) expression()                {}
func (n *CallExpr) String() string {
	return nodeString(n.Function) + "(" + joinExpressions(n.Arguments, ", ") + ")"
}

type IndexExpr struct {
	Value Expression
	Open  token.Token
	Index Expression
}

func (n *IndexExpr) Position() token.Position { return n.Value.Position() }
func (*IndexExpr) expression()                {}
func (n *IndexExpr) String() string           { return nodeString(n.Value) + "[" + nodeString(n.Index) + "]" }

type MemberExpr struct {
	Value    Expression
	Operator token.Token
	Member   token.Token
}

func (n *MemberExpr) Position() token.Position { return n.Value.Position() }
func (*MemberExpr) expression()                {}
func (n *MemberExpr) String() string {
	return nodeString(n.Value) + n.Operator.Literal + n.Member.Literal
}

type CastExpr struct {
	Open       token.Token
	Type       []TypeSpec
	Declarator Declarator
	Value      Expression
}

func (n *CastExpr) Position() token.Position { return n.Open.Pos }
func (*CastExpr) expression()                {}
func (n *CastExpr) String() string           { return "(" + typeSpecsString(n.Type) + ")" + nodeString(n.Value) }

type SizeofExpr struct {
	Token      token.Token
	Type       []TypeSpec
	Declarator Declarator
	Value      Expression
}

func (n *SizeofExpr) Position() token.Position { return n.Token.Pos }
func (*SizeofExpr) expression()                {}
func (n *SizeofExpr) String() string           { return n.Token.Literal + " " + nodeString(n.Value) }

type CommaExpr struct{ Expressions []Expression }

func (n *CommaExpr) Position() token.Position { return n.Expressions[0].Position() }
func (*CommaExpr) expression()                {}
func (n *CommaExpr) String() string           { return joinExpressions(n.Expressions, ", ") }

type CompoundLiteralExpr struct {
	Open        token.Token
	Type        []TypeSpec
	Declarator  Declarator
	Initializer Expression
}

func (n *CompoundLiteralExpr) Position() token.Position { return n.Open.Pos }
func (*CompoundLiteralExpr) expression()                {}
func (n *CompoundLiteralExpr) String() string {
	return "(" + typeSpecsString(n.Type) + ")" + nodeString(n.Initializer)
}

type InitializerListExpr struct {
	Open   token.Token
	Values []InitializerElement
}

func (n *InitializerListExpr) Position() token.Position { return n.Open.Pos }
func (*InitializerListExpr) expression()                {}
func (n *InitializerListExpr) String() string {
	values := make([]string, len(n.Values))
	for i, value := range n.Values {
		prefix := ""
		for _, d := range value.Designators {
			if d.Token.Type == token.DOT {
				prefix += "." + d.Field.Literal
			} else if d.Token.Type == token.LBRACK {
				prefix += "[" + nodeString(d.Index) + "]"
			}
		}
		if prefix != "" {
			prefix += " = "
		}
		values[i] = prefix + nodeString(value.Value)
	}
	return "{" + strings.Join(values, ", ") + "}"
}

func nodeString(n Node) string {
	if n == nil {
		return ""
	}
	return n.String()
}
func joinNodes(nodes []Node, separator string) string {
	values := make([]string, len(nodes))
	for i, node := range nodes {
		values[i] = nodeString(node)
	}
	return strings.Join(values, separator)
}
func declarationsToNodes(declarations []Declaration) []Node {
	nodes := make([]Node, len(declarations))
	for i, declaration := range declarations {
		nodes[i] = declaration
	}
	return nodes
}
func joinExpressions(expressions []Expression, separator string) string {
	nodes := make([]Node, len(expressions))
	for i, expression := range expressions {
		nodes[i] = expression
	}
	return joinNodes(nodes, separator)
}
func typeSpecsString(specs []TypeSpec) string {
	values := make([]string, len(specs))
	for i, spec := range specs {
		values[i] = spec.Token.Literal
	}
	return strings.Join(values, " ")
}
func declaratorsString(declarators []Declarator) string {
	values := make([]string, len(declarators))
	for i, declarator := range declarators {
		values[i] = declarator.Name.Literal
		if declarator.Initializer != nil {
			values[i] += " = " + declarator.Initializer.String()
		}
	}
	return strings.Join(values, ", ")
}
func elseString(statement Statement) string {
	if statement == nil {
		return ""
	}
	return " else " + statement.String()
}

type InitializerElement struct {
	Designators []Designator
	Value       Expression
}
type Designator struct {
	Token token.Token
	Field token.Token
	Index Expression
}
