// Package parser turns Scintilla's (C99) token stream into an abstract syntax tree.
package parser

import "tvshow/lang/token"

// Node is implemented by every AST node. Pos is the first token belonging to it.
type Node interface{ Position() token.Position }
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
	Name        token.Token
	Pointers    []Pointer
	Suffixes    []DeclaratorSuffix
	Initializer Expression
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

type FunctionSuffix struct {
	Open       token.Token
	Parameters []Parameter
	Variadic   bool
}

func (n *FunctionSuffix) Position() token.Position { return n.Open.Pos }
func (*FunctionSuffix) declaratorSuffix()          {}

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

type FunctionDecl struct {
	Specs      []TypeSpec
	Declarator Declarator
	Body       *BlockStmt
}

func (n *FunctionDecl) Position() token.Position { return n.Specs[0].Position() }
func (*FunctionDecl) declaration()               {}

type StaticAssertDecl struct {
	Token     token.Token
	Condition Expression
	Message   token.Token
}

func (n *StaticAssertDecl) Position() token.Position { return n.Token.Pos }
func (*StaticAssertDecl) declaration()               {}

type BlockStmt struct {
	Open  token.Token
	Items []Node
	Close token.Token
}

func (n *BlockStmt) Position() token.Position { return n.Open.Pos }
func (*BlockStmt) statement()                 {}

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
func (*ExprStmt) statement() {}

type IfStmt struct {
	Token      token.Token
	Condition  Expression
	Then, Else Statement
}

func (n *IfStmt) Position() token.Position { return n.Token.Pos }
func (*IfStmt) statement()                 {}

type SwitchStmt struct {
	Token token.Token
	Value Expression
	Body  Statement
}

func (n *SwitchStmt) Position() token.Position { return n.Token.Pos }
func (*SwitchStmt) statement()                 {}

type WhileStmt struct {
	Token     token.Token
	Condition Expression
	Body      Statement
}

func (n *WhileStmt) Position() token.Position { return n.Token.Pos }
func (*WhileStmt) statement()                 {}

type DoWhileStmt struct {
	Token     token.Token
	Body      Statement
	Condition Expression
}

func (n *DoWhileStmt) Position() token.Position { return n.Token.Pos }
func (*DoWhileStmt) statement()                 {}

type ForStmt struct {
	Token           token.Token
	Init            Node
	Condition, Post Expression
	Body            Statement
}

func (n *ForStmt) Position() token.Position { return n.Token.Pos }
func (*ForStmt) statement()                 {}

type JumpStmt struct {
	Token token.Token
	Label token.Token
	Value Expression
	Semi  token.Token
}

func (n *JumpStmt) Position() token.Position { return n.Token.Pos }
func (*JumpStmt) statement()                 {}

type LabelStmt struct {
	Name      token.Token
	Colon     token.Token
	Statement Statement
}

func (n *LabelStmt) Position() token.Position { return n.Name.Pos }
func (*LabelStmt) statement()                 {}

type CaseStmt struct {
	Token token.Token
	Value Expression
	Body  Statement
}

func (n *CaseStmt) Position() token.Position { return n.Token.Pos }
func (*CaseStmt) statement()                 {}

type IdentExpr struct{ Token token.Token }

func (n *IdentExpr) Position() token.Position { return n.Token.Pos }
func (*IdentExpr) expression()                {}

type LiteralExpr struct{ Token token.Token }

func (n *LiteralExpr) Position() token.Position { return n.Token.Pos }
func (*LiteralExpr) expression()                {}

type UnaryExpr struct {
	Operator token.Token
	Operand  Expression
	Postfix  bool
}

func (n *UnaryExpr) Position() token.Position { return n.Operator.Pos }
func (*UnaryExpr) expression()                {}

type BinaryExpr struct {
	Left     Expression
	Operator token.Token
	Right    Expression
}

func (n *BinaryExpr) Position() token.Position { return n.Left.Position() }
func (*BinaryExpr) expression()                {}

type ConditionalExpr struct {
	Condition Expression
	Question  token.Token
	Then      Expression
	Else      Expression
}

func (n *ConditionalExpr) Position() token.Position { return n.Condition.Position() }
func (*ConditionalExpr) expression()                {}

type AssignExpr struct {
	Left     Expression
	Operator token.Token
	Right    Expression
}

func (n *AssignExpr) Position() token.Position { return n.Left.Position() }
func (*AssignExpr) expression()                {}

type CallExpr struct {
	Function  Expression
	Open      token.Token
	Arguments []Expression
}

func (n *CallExpr) Position() token.Position { return n.Function.Position() }
func (*CallExpr) expression()                {}

type IndexExpr struct {
	Value Expression
	Open  token.Token
	Index Expression
}

func (n *IndexExpr) Position() token.Position { return n.Value.Position() }
func (*IndexExpr) expression()                {}

type MemberExpr struct {
	Value    Expression
	Operator token.Token
	Member   token.Token
}

func (n *MemberExpr) Position() token.Position { return n.Value.Position() }
func (*MemberExpr) expression()                {}

type CastExpr struct {
	Open       token.Token
	Type       []TypeSpec
	Declarator Declarator
	Value      Expression
}

func (n *CastExpr) Position() token.Position { return n.Open.Pos }
func (*CastExpr) expression()                {}

type SizeofExpr struct {
	Token      token.Token
	Type       []TypeSpec
	Declarator Declarator
	Value      Expression
}

func (n *SizeofExpr) Position() token.Position { return n.Token.Pos }
func (*SizeofExpr) expression()                {}

type CommaExpr struct{ Expressions []Expression }

func (n *CommaExpr) Position() token.Position { return n.Expressions[0].Position() }
func (*CommaExpr) expression()                {}

type CompoundLiteralExpr struct {
	Open        token.Token
	Type        []TypeSpec
	Declarator  Declarator
	Initializer Expression
}

func (n *CompoundLiteralExpr) Position() token.Position { return n.Open.Pos }
func (*CompoundLiteralExpr) expression()                {}

type InitializerListExpr struct {
	Open   token.Token
	Values []InitializerElement
}

func (n *InitializerListExpr) Position() token.Position { return n.Open.Pos }
func (*InitializerListExpr) expression()                {}

type InitializerElement struct {
	Designators []Designator
	Value       Expression
}
type Designator struct {
	Token token.Token
	Field token.Token
	Index Expression
}
