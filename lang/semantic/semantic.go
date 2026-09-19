// Package semantic validates Scintilla abstract syntax trees before they are
// translated to bytecode.  It deliberately works on the parser's public AST,
// so embedders can run it on trees they construct themselves as well.
package semantic

import (
	"fmt"
	"strings"

	"tvshow/lang/parser"
	"tvshow/lang/token"
)

// Error describes one semantic problem at a source position.
type Error struct {
	Pos     token.Position
	Message string
}

func (e Error) Error() string { return fmt.Sprintf("%s: %s", e.Pos, e.Message) }

// Errors is returned when analysis finds more than one problem.
type Errors []Error

func (e Errors) Error() string {
	lines := make([]string, len(e))
	for i, problem := range e {
		lines[i] = problem.Error()
	}
	return strings.Join(lines, "\n")
}

// Analyzer holds the state used for one analysis.  A new Analyzer must be
// used for each translation unit.
type Analyzer struct {
	values *scope
	types  *scope
	errs   Errors

	function *functionContext
	loops    int
	switches int
}

type symbolKind uint8

const (
	variableSymbol symbolKind = iota
	functionSymbol
	typedefSymbol
	enumeratorSymbol
)

type symbol struct{ kind symbolKind }
type scope struct {
	parent  *scope
	entries map[string]symbol
}
type functionContext struct {
	labels map[string]token.Position
	gotos  []parser.JumpStmt
}

// New returns an initialized analyzer.
func New() *Analyzer {
	return &Analyzer{values: newScope(nil), types: newScope(nil)}
}

// Analyze checks program and returns all errors found, if any.
func Analyze(program *parser.Program) error { return New().Analyze(program) }

// Analyze checks program and returns all errors found, if any.
func (a *Analyzer) Analyze(program *parser.Program) error {
	if program == nil {
		return Error{Message: "cannot analyze a nil program"}
	}
	a.predeclareFunctions(program)
	for _, declaration := range program.Declarations {
		a.declaration(declaration, true)
	}
	if len(a.errs) != 0 {
		return a.errs
	}
	return nil
}

func newScope(parent *scope) *scope { return &scope{parent: parent, entries: make(map[string]symbol)} }
func (s *scope) lookup(name string) (symbol, bool) {
	for ; s != nil; s = s.parent {
		if symbol, ok := s.entries[name]; ok {
			return symbol, true
		}
	}
	return symbol{}, false
}
func (a *Analyzer) problem(pos token.Position, format string, args ...any) {
	a.errs = append(a.errs, Error{Pos: pos, Message: fmt.Sprintf(format, args...)})
}
func (a *Analyzer) pushScope() { a.values = newScope(a.values); a.types = newScope(a.types) }
func (a *Analyzer) popScope()  { a.values, a.types = a.values.parent, a.types.parent }

func (a *Analyzer) predeclareFunctions(program *parser.Program) {
	for _, declaration := range program.Declarations {
		if function, ok := declaration.(*parser.FunctionDecl); ok && function.Declarator.Name.Literal != "" {
			name := function.Declarator.Name.Literal
			if old, exists := a.values.entries[name]; exists && old.kind != functionSymbol {
				a.problem(function.Declarator.Name.Pos, "redefinition of %q", name)
			} else {
				a.values.entries[name] = symbol{functionSymbol}
			}
		}
	}
}

func (a *Analyzer) declaration(declaration parser.Declaration, global bool) {
	switch d := declaration.(type) {
	case *parser.FunctionDecl:
		a.specs(d.Specs)
		if d.Body != nil {
			a.functionDecl(d)
		}
	case *parser.VarDecl:
		a.specs(d.Specs)
		isTypedef := hasSpec(d.Specs, token.TYPEDEF)
		for _, declarator := range d.Declarators {
			name := declarator.Name.Literal
			if name == "" {
				a.declarator(declarator)
				continue
			}
			destination := a.values
			kind := variableSymbol
			if isTypedef {
				destination, kind = a.types, typedefSymbol
			}
			if _, exists := destination.entries[name]; exists {
				a.problem(declarator.Name.Pos, "redefinition of %q", name)
			} else {
				destination.entries[name] = symbol{kind}
			}
			a.declarator(declarator)
		}
	case *parser.StaticAssertDecl:
		a.expression(d.Condition)
	}
}

func hasSpec(specs []parser.TypeSpec, wanted token.TokenType) bool {
	for _, spec := range specs {
		if spec.Token.Type == wanted {
			return true
		}
	}
	return false
}
func (a *Analyzer) specs(specs []parser.TypeSpec) {
	for _, spec := range specs {
		if spec.Token.Type == token.IDENT {
			if _, ok := a.types.lookup(spec.Token.Literal); !ok {
				a.problem(spec.Token.Pos, "unknown type %q", spec.Token.Literal)
			}
		}
		// Members belong to the aggregate, not the enclosing identifier scope.
		if len(spec.Members) != 0 {
			a.pushScope()
			for _, member := range spec.Members {
				a.declaration(member, false)
			}
			a.popScope()
		}
		for _, value := range spec.EnumValues {
			if value.Value != nil {
				a.expression(value.Value)
			}
			if _, exists := a.values.entries[value.Name.Literal]; exists {
				a.problem(value.Name.Pos, "redefinition of %q", value.Name.Literal)
			} else {
				a.values.entries[value.Name.Literal] = symbol{enumeratorSymbol}
			}
		}
	}
}
func (a *Analyzer) declarator(d parser.Declarator) {
	for _, suffix := range d.Suffixes {
		switch suffix := suffix.(type) {
		case *parser.ArraySuffix:
			a.expression(suffix.Size)
		case *parser.FunctionSuffix:
			for _, parameter := range suffix.Parameters {
				a.specs(parameter.Specs)
				a.declarator(parameter.Declarator)
			}
		}
	}
	if d.Initializer != nil {
		a.expression(d.Initializer)
	}
}

func (a *Analyzer) functionDecl(d *parser.FunctionDecl) {
	previous := a.function
	a.function = &functionContext{labels: make(map[string]token.Position)}
	a.pushScope()
	for _, suffix := range d.Declarator.Suffixes {
		function, ok := suffix.(*parser.FunctionSuffix)
		if !ok {
			continue
		}
		for _, parameter := range function.Parameters {
			a.specs(parameter.Specs)
			name := parameter.Declarator.Name
			if name.Literal == "" {
				continue
			}
			if _, exists := a.values.entries[name.Literal]; exists {
				a.problem(name.Pos, "redefinition of parameter %q", name.Literal)
			} else {
				a.values.entries[name.Literal] = symbol{variableSymbol}
			}
		}
	}
	a.block(d.Body, false)
	for _, jump := range a.function.gotos {
		if _, ok := a.function.labels[jump.Label.Literal]; !ok {
			a.problem(jump.Label.Pos, "undefined label %q", jump.Label.Literal)
		}
	}
	a.popScope()
	a.function = previous
}

func (a *Analyzer) block(block *parser.BlockStmt, scoped bool) {
	if block == nil {
		return
	}
	if scoped {
		a.pushScope()
		defer a.popScope()
	}
	for _, item := range block.Items {
		switch item := item.(type) {
		case parser.Declaration:
			a.declaration(item, false)
		case parser.Statement:
			a.statement(item)
		}
	}
}
func (a *Analyzer) statement(statement parser.Statement) {
	switch s := statement.(type) {
	case *parser.BlockStmt:
		a.block(s, true)
	case *parser.ExprStmt:
		a.expression(s.Expr)
	case *parser.IfStmt:
		a.expression(s.Condition)
		a.statement(s.Then)
		a.statement(s.Else)
	case *parser.WhileStmt:
		a.expression(s.Condition)
		a.loops++
		a.statement(s.Body)
		a.loops--
	case *parser.DoWhileStmt:
		a.loops++
		a.statement(s.Body)
		a.loops--
		a.expression(s.Condition)
	case *parser.ForStmt:
		a.pushScope()
		if d, ok := s.Init.(parser.Declaration); ok {
			a.declaration(d, false)
		} else if x, ok := s.Init.(parser.Statement); ok {
			a.statement(x)
		}
		a.expression(s.Condition)
		a.expression(s.Post)
		a.loops++
		a.statement(s.Body)
		a.loops--
		a.popScope()
	case *parser.SwitchStmt:
		a.expression(s.Value)
		a.switches++
		a.statement(s.Body)
		a.switches--
	case *parser.CaseStmt:
		if a.switches == 0 {
			a.problem(s.Token.Pos, "%s statement is not within a switch", s.Token.Literal)
		}
		a.expression(s.Value)
		a.statement(s.Body)
	case *parser.LabelStmt:
		if _, exists := a.function.labels[s.Name.Literal]; exists {
			a.problem(s.Name.Pos, "duplicate label %q", s.Name.Literal)
		} else {
			a.function.labels[s.Name.Literal] = s.Name.Pos
		}
		a.statement(s.Statement)
	case *parser.JumpStmt:
		switch s.Token.Type {
		case token.BREAK:
			if a.loops == 0 && a.switches == 0 {
				a.problem(s.Token.Pos, "break statement is not within a loop or switch")
			}
		case token.CONTINUE:
			if a.loops == 0 {
				a.problem(s.Token.Pos, "continue statement is not within a loop")
			}
		case token.GOTO:
			a.function.gotos = append(a.function.gotos, *s)
		}
		a.expression(s.Value)
	}
}
func (a *Analyzer) expression(expression parser.Expression) {
	if expression == nil {
		return
	}
	switch e := expression.(type) {
	case *parser.IdentExpr:
		if _, ok := a.values.lookup(e.Token.Literal); !ok {
			a.problem(e.Token.Pos, "undefined identifier %q", e.Token.Literal)
		}
	case *parser.UnaryExpr:
		a.expression(e.Operand)
	case *parser.BinaryExpr:
		a.expression(e.Left)
		a.expression(e.Right)
	case *parser.AssignExpr:
		if !assignable(e.Left) {
			a.problem(e.Left.Position(), "left operand of %s is not assignable", e.Operator.Literal)
		}
		a.expression(e.Left)
		a.expression(e.Right)
	case *parser.ConditionalExpr:
		a.expression(e.Condition)
		a.expression(e.Then)
		a.expression(e.Else)
	case *parser.CallExpr:
		a.expression(e.Function)
		for _, argument := range e.Arguments {
			a.expression(argument)
		}
	case *parser.IndexExpr:
		a.expression(e.Value)
		a.expression(e.Index)
	case *parser.MemberExpr:
		a.expression(e.Value)
	case *parser.CastExpr:
		a.specs(e.Type)
		a.declarator(e.Declarator)
		a.expression(e.Value)
	case *parser.SizeofExpr:
		a.specs(e.Type)
		a.declarator(e.Declarator)
		a.expression(e.Value)
	case *parser.CommaExpr:
		for _, value := range e.Expressions {
			a.expression(value)
		}
	case *parser.CompoundLiteralExpr:
		a.specs(e.Type)
		a.declarator(e.Declarator)
		a.expression(e.Initializer)
	case *parser.InitializerListExpr:
		for _, value := range e.Values {
			for _, designator := range value.Designators {
				a.expression(designator.Index)
			}
			a.expression(value.Value)
		}
	}
}
func assignable(expression parser.Expression) bool {
	switch expression.(type) {
	case *parser.IdentExpr, *parser.IndexExpr, *parser.MemberExpr, *parser.CompoundLiteralExpr:
		return true
	case *parser.UnaryExpr:
		return expression.(*parser.UnaryExpr).Operator.Type == token.ASTERISK
	}
	return false
}
