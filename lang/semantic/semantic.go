// Package semantic validates Scintilla abstract syntax trees before they are
// translated to bytecode.  It deliberately works on the parser's public AST,
// so embedders can run it on trees they construct themselves as well.
package semantic

import (
	"fmt"
	"strconv"
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

type TypeKind uint8

const (
	TypeUnknown TypeKind = iota
	TypeVoid
	TypeInt
	TypeFloat
	TypeChar
	TypeString
	TypeBool
	TypePointer
	TypeArray
	TypeStruct
	TypeUnion
	TypeFunction
	TypeAuto
)

type Type struct {
	Kind       TypeKind
	IsConst    bool
	Base       *Type          // For Pointer (target) or Array (element)
	ArraySize  int            // Fixed size if known, else 0
	HasSize    bool           // whether size was explicitly specified in array suffix
	Members    []StructMember // For Struct or Union
	Tag        string         // Tag or name for struct/union/typedef
	Params     []Type         // For Function
	ReturnType *Type          // For Function
	Variadic   bool           // For Function
}

type StructMember struct {
	Name    string
	Type    Type
	IsConst bool
	Pos     token.Position
}

func (t Type) String() string {
	var prefix string
	if t.IsConst {
		prefix = "const "
	}
	switch t.Kind {
	case TypeVoid:
		return prefix + "void"
	case TypeInt:
		return prefix + "int"
	case TypeFloat:
		return prefix + "float"
	case TypeChar:
		return prefix + "char"
	case TypeString:
		return prefix + "string"
	case TypeBool:
		return prefix + "bool"
	case TypePointer:
		if t.Base != nil {
			return t.Base.String() + " *"
		}
		return prefix + "pointer"
	case TypeArray:
		if t.Base != nil {
			if t.HasSize {
				return fmt.Sprintf("%s[%d]", t.Base.String(), t.ArraySize)
			}
			return t.Base.String() + "[]"
		}
		return prefix + "array"
	case TypeStruct:
		if t.Tag != "" {
			return prefix + "struct " + t.Tag
		}
		return prefix + "struct"
	case TypeUnion:
		if t.Tag != "" {
			return prefix + "union " + t.Tag
		}
		return prefix + "union"
	case TypeFunction:
		return prefix + "function"
	case TypeAuto:
		return prefix + "auto"
	default:
		return "unknown"
	}
}

type symbolKind uint8

const (
	variableSymbol symbolKind = iota
	functionSymbol
	typedefSymbol
	enumeratorSymbol
)

type symbol struct {
	kind symbolKind
	typ  Type
}

type scope struct {
	parent  *scope
	entries map[string]symbol
}

type functionContext struct {
	returnType Type
	labels     map[string]token.Position
	gotos      []parser.JumpStmt
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

func newScope(parent *scope) *scope {
	return &scope{parent: parent, entries: make(map[string]symbol)}
}

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
			retBase := a.typeFromSpecs(function.Specs)
			funcType := a.typeFromDeclarator(retBase, function.Declarator)
			if old, exists := a.values.entries[name]; exists && old.kind != functionSymbol {
				a.problem(function.Declarator.Name.Pos, "redefinition of %q", name)
			} else {
				a.values.entries[name] = symbol{kind: functionSymbol, typ: funcType}
			}
		}
	}
}

func (a *Analyzer) declaration(declaration parser.Declaration, global bool) {
	switch d := declaration.(type) {
	case *parser.FunctionDecl:
		a.specs(d.Specs)
		name := d.Declarator.Name.Literal
		retBase := a.typeFromSpecs(d.Specs)
		funcType := a.typeFromDeclarator(retBase, d.Declarator)
		if name != "" {
			a.values.entries[name] = symbol{kind: functionSymbol, typ: funcType}
		}
		if d.Body != nil {
			a.functionDecl(d)
		}
	case *parser.VarDecl:
		a.specs(d.Specs)
		isTypedef := hasSpec(d.Specs, token.TYPEDEF)
		baseType := a.typeFromSpecs(d.Specs)

		for _, declarator := range d.Declarators {
			name := declarator.Name.Literal
			vType := a.typeFromDeclarator(baseType, declarator)

			if name != "" {
				destination := a.values
				kind := variableSymbol
				if isTypedef {
					destination, kind = a.types, typedefSymbol
				} else if vType.Kind == TypeFunction {
					kind = functionSymbol
				}

				if old, exists := destination.entries[name]; exists && !(kind == functionSymbol && old.kind == functionSymbol) {
					a.problem(declarator.Name.Pos, "redefinition of %q", name)
				} else {
					destination.entries[name] = symbol{kind: kind, typ: vType}
				}
			}

			if !isTypedef && vType.Kind != TypeFunction {
				if vType.Kind == TypeVoid {
					a.problem(declarator.Name.Pos, "variable %q declared with void type", name)
				}
				if vType.Kind == TypeArray && vType.Base != nil && vType.Base.Kind == TypeVoid {
					a.problem(declarator.Name.Pos, "array %q element cannot be void", name)
				}
				if vType.IsConst && declarator.Initializer == nil && !global {
					a.problem(declarator.Name.Pos, "uninitialized const variable %q", name)
				}
			}

			a.declarator(declarator, vType)
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
				a.values.entries[value.Name.Literal] = symbol{kind: enumeratorSymbol, typ: Type{Kind: TypeInt}}
			}
		}
	}
}

func (a *Analyzer) typeFromSpecs(specs []parser.TypeSpec) Type {
	t := Type{Kind: TypeInt}
	isConst := false

	for _, spec := range specs {
		if spec.Token.Type == token.CONST {
			isConst = true
		}
		for _, q := range spec.Qualifiers {
			if q.Type == token.CONST {
				isConst = true
			}
		}

		switch spec.Token.Type {
		case token.AUTO:
			t.Kind = TypeAuto
		case token.VOID:
			t.Kind = TypeVoid
		case token.INT_KW, token.SHORT, token.LONG, token.SIGNED, token.UNSIGNED:
			t.Kind = TypeInt
		case token.FLOAT_KW, token.DOUBLE:
			t.Kind = TypeFloat
		case token.CHAR_KW:
			t.Kind = TypeChar
		case token.STRING_KW:
			t.Kind = TypeString
		case token.BOOL:
			t.Kind = TypeBool
		case token.STRUCT, token.UNION:
			if spec.Token.Type == token.STRUCT {
				t.Kind = TypeStruct
			} else {
				t.Kind = TypeUnion
			}
			t.Tag = spec.Tag

			if len(spec.Members) > 0 {
				var members []StructMember
				fieldsSeen := make(map[string]token.Position)
				for _, member := range spec.Members {
					if vd, ok := member.(*parser.VarDecl); ok {
						mTypeBase := a.typeFromSpecs(vd.Specs)
						for _, decl := range vd.Declarators {
							mType := a.typeFromDeclarator(mTypeBase, decl)
							mName := decl.Name.Literal
							if mName != "" {
								if prevPos, exists := fieldsSeen[mName]; exists {
									a.problem(decl.Name.Pos, "duplicate member %q (previously declared at %s)", mName, prevPos)
								} else {
									fieldsSeen[mName] = decl.Name.Pos
								}
								if mType.Kind == TypeVoid {
									a.problem(decl.Name.Pos, "member %q has invalid void type", mName)
								}
								members = append(members, StructMember{
									Name:    mName,
									Type:    mType,
									IsConst: mType.IsConst || mTypeBase.IsConst,
									Pos:     decl.Name.Pos,
								})
							}
						}
					}
				}
				t.Members = members
				if spec.Tag != "" {
					key := "struct " + spec.Tag
					if spec.Token.Type == token.UNION {
						key = "union " + spec.Tag
					}
					a.types.entries[key] = symbol{kind: typedefSymbol, typ: t}
				}
			} else if spec.Tag != "" {
				key := "struct " + spec.Tag
				if spec.Token.Type == token.UNION {
					key = "union " + spec.Tag
				}
				if sym, ok := a.types.lookup(key); ok {
					t.Members = sym.typ.Members
				} else if sym, ok := a.types.lookup(spec.Tag); ok {
					t.Members = sym.typ.Members
				}
			}
		case token.ENUM:
			t.Kind = TypeInt
			t.Tag = spec.Tag
		case token.IDENT:
			if sym, ok := a.types.lookup(spec.Token.Literal); ok {
				t = sym.typ
			}
		}
	}

	if isConst {
		t.IsConst = true
	}
	return t
}

func (a *Analyzer) typeFromDeclarator(baseType Type, declarator parser.Declarator) Type {
	curType := baseType

	for _, ptr := range declarator.Pointers {
		ptrConst := false
		for _, q := range ptr.Qualifiers {
			if q.Type == token.CONST {
				ptrConst = true
			}
		}
		target := curType
		curType = Type{
			Kind:    TypePointer,
			IsConst: ptrConst,
			Base:    &target,
		}
	}

	for _, suffix := range declarator.Suffixes {
		switch s := suffix.(type) {
		case *parser.ArraySuffix:
			elemType := curType
			arrSize := 0
			hasSize := false
			if s.Size != nil {
				hasSize = true
				sizeType := a.expression(s.Size)
				if sizeType.Kind != TypeUnknown && !isIntegerType(sizeType.Kind) {
					a.problem(s.Size.Position(), "size of array has non-integer type")
				}
				if n, ok := evalConstInt(s.Size); ok {
					if n < 0 {
						a.problem(s.Size.Position(), "size of array is negative")
					} else {
						arrSize = n
					}
				}
			}
			curType = Type{
				Kind:      TypeArray,
				IsConst:   elemType.IsConst,
				Base:      &elemType,
				ArraySize: arrSize,
				HasSize:   hasSize,
			}
		case *parser.FunctionSuffix:
			retType := curType
			var params []Type
			for _, p := range s.Parameters {
				pBase := a.typeFromSpecs(p.Specs)
				pType := a.typeFromDeclarator(pBase, p.Declarator)

				if pType.Kind == TypeVoid && p.Declarator.Name.Literal != "" {
					a.problem(p.Declarator.Name.Pos, "parameter %q declared with void type", p.Declarator.Name.Literal)
				}
				if pType.Kind == TypeVoid && p.Declarator.Name.Literal == "" && len(s.Parameters) == 1 {
					continue
				}
				params = append(params, pType)
			}
			curType = Type{
				Kind:       TypeFunction,
				ReturnType: &retType,
				Params:     params,
				Variadic:   s.Variadic,
			}
		}
	}

	for _, ptr := range declarator.FuncPointers {
		ptrConst := false
		for _, q := range ptr.Qualifiers {
			if q.Type == token.CONST {
				ptrConst = true
			}
		}
		target := curType
		curType = Type{
			Kind:    TypePointer,
			IsConst: ptrConst,
			Base:    &target,
		}
	}

	return curType
}

func evalConstInt(e parser.Expression) (int, bool) {
	if e == nil {
		return 0, false
	}
	switch e := e.(type) {
	case *parser.LiteralExpr:
		if e.Token.Type == token.INT {
			if n, err := strconv.Atoi(e.Token.Literal); err == nil {
				return n, true
			}
		}
	case *parser.UnaryExpr:
		if e.Operator.Type == token.MINUS {
			if n, ok := evalConstInt(e.Operand); ok {
				return -n, true
			}
		}
		if e.Operator.Type == token.PLUS {
			return evalConstInt(e.Operand)
		}
	}
	return 0, false
}

func (a *Analyzer) declarator(d parser.Declarator, declType Type) {
	for _, suffix := range d.Suffixes {
		switch suffix := suffix.(type) {
		case *parser.ArraySuffix:
			if suffix.Size != nil {
				a.expression(suffix.Size)
			}
		case *parser.FunctionSuffix:
			for _, parameter := range suffix.Parameters {
				a.specs(parameter.Specs)
				a.declarator(parameter.Declarator, a.typeFromDeclarator(a.typeFromSpecs(parameter.Specs), parameter.Declarator))
			}
		}
	}
	if d.Initializer != nil {
		initType := a.expression(d.Initializer)
		if declType.Kind != TypeUnknown && !a.isCompatible(declType, initType) {
			a.problem(d.Initializer.Position(), "incompatible type in initialization of %q", d.Name.Literal)
		}
		if declType.Kind == TypeArray && declType.HasSize && declType.ArraySize > 0 {
			if initList, ok := d.Initializer.(*parser.InitializerListExpr); ok {
				if len(initList.Values) > declType.ArraySize {
					a.problem(d.Initializer.Position(), "excess elements in array initializer for %q", d.Name.Literal)
				}
			}
		}
	}
}

func (a *Analyzer) functionDecl(d *parser.FunctionDecl) {
	previous := a.function
	retBase := a.typeFromSpecs(d.Specs)
	funcType := a.typeFromDeclarator(retBase, d.Declarator)

	retType := Type{Kind: TypeVoid}
	if funcType.ReturnType != nil {
		retType = *funcType.ReturnType
	}

	a.function = &functionContext{returnType: retType, labels: make(map[string]token.Position)}
	a.pushScope()

	for _, suffix := range d.Declarator.Suffixes {
		function, ok := suffix.(*parser.FunctionSuffix)
		if !ok {
			continue
		}
		for _, parameter := range function.Parameters {
			a.specs(parameter.Specs)
			pBase := a.typeFromSpecs(parameter.Specs)
			pType := a.typeFromDeclarator(pBase, parameter.Declarator)
			name := parameter.Declarator.Name

			if pType.Kind == TypeVoid && name.Literal != "" {
				a.problem(name.Pos, "parameter %q declared with void type", name.Literal)
			}

			if name.Literal == "" {
				continue
			}
			if _, exists := a.values.entries[name.Literal]; exists {
				a.problem(name.Pos, "redefinition of parameter %q", name.Literal)
			} else {
				a.values.entries[name.Literal] = symbol{kind: variableSymbol, typ: pType}
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
		cType := a.expression(s.Condition)
		if cType.Kind == TypeVoid {
			a.problem(s.Condition.Position(), "void value in condition")
		}
		a.statement(s.Then)
		a.statement(s.Else)
	case *parser.WhileStmt:
		cType := a.expression(s.Condition)
		if cType.Kind == TypeVoid {
			a.problem(s.Condition.Position(), "void value in condition")
		}
		a.loops++
		a.statement(s.Body)
		a.loops--
	case *parser.DoWhileStmt:
		a.loops++
		a.statement(s.Body)
		a.loops--
		cType := a.expression(s.Condition)
		if cType.Kind == TypeVoid {
			a.problem(s.Condition.Position(), "void value in condition")
		}
	case *parser.ForStmt:
		a.pushScope()
		if d, ok := s.Init.(parser.Declaration); ok {
			a.declaration(d, false)
		} else if x, ok := s.Init.(parser.Statement); ok {
			a.statement(x)
		}
		if s.Condition != nil {
			cType := a.expression(s.Condition)
			if cType.Kind == TypeVoid {
				a.problem(s.Condition.Position(), "void value in condition")
			}
		}
		a.expression(s.Post)
		a.loops++
		a.statement(s.Body)
		a.loops--
		a.popScope()
	case *parser.SwitchStmt:
		vType := a.expression(s.Value)
		if vType.Kind != TypeUnknown && vType.Kind != TypeAuto && !isIntegerType(vType.Kind) && vType.Kind != TypeString {
			a.problem(s.Value.Position(), "switch quantity is not an integer")
		}
		a.switches++
		a.statement(s.Body)
		a.switches--
	case *parser.CaseStmt:
		if a.switches == 0 {
			a.problem(s.Token.Pos, "%s statement is not within a switch", s.Token.Literal)
		}
		if s.Value != nil {
			cType := a.expression(s.Value)
			if cType.Kind != TypeUnknown && cType.Kind != TypeAuto && !isIntegerType(cType.Kind) && cType.Kind != TypeString {
				a.problem(s.Value.Position(), "case label is not an integer")
			}
		}
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
		case token.RETURN:
			if a.function != nil {
				retType := a.function.returnType
				if retType.Kind == TypeVoid && s.Value != nil {
					a.problem(s.Token.Pos, "void function should not return a value")
				} else if retType.Kind != TypeVoid && s.Value == nil {
					a.problem(s.Token.Pos, "non-void function should return a value")
				} else if s.Value != nil {
					vType := a.expression(s.Value)
					if vType.Kind != TypeUnknown && !a.isCompatible(retType, vType) {
						a.problem(s.Value.Position(), "incompatible return type in function returning %s", retType.String())
					}
				}
			}
		}
		if s.Token.Type != token.RETURN {
			a.expression(s.Value)
		}
	}
}

func (a *Analyzer) expression(expression parser.Expression) Type {
	if expression == nil {
		return Type{Kind: TypeUnknown}
	}
	switch e := expression.(type) {
	case *parser.IdentExpr:
		if sym, ok := a.values.lookup(e.Token.Literal); ok {
			return sym.typ
		}
		a.problem(e.Token.Pos, "undefined identifier %q", e.Token.Literal)
		return Type{Kind: TypeUnknown}

	case *parser.LiteralExpr:
		switch e.Token.Type {
		case token.INT:
			return Type{Kind: TypeInt}
		case token.FLOAT:
			return Type{Kind: TypeFloat}
		case token.CHAR:
			return Type{Kind: TypeChar}
		case token.STRING:
			return Type{Kind: TypeString}
		default:
			return Type{Kind: TypeInt}
		}

	case *parser.UnaryExpr:
		t := a.expression(e.Operand)
		switch e.Operator.Type {
		case token.PLUS, token.MINUS:
			if t.Kind != TypeUnknown && t.Kind != TypeAuto && !isNumericType(t.Kind) {
				a.problem(e.Operator.Pos, "invalid operand of type %q to unary %s", t.String(), e.Operator.Literal)
			}
			return t
		case token.LOGICAL_NOT:
			if t.Kind == TypeVoid {
				a.problem(e.Operator.Pos, "void operand to logical NOT")
			}
			return Type{Kind: TypeBool}
		case token.BIT_NOT:
			if t.Kind != TypeUnknown && t.Kind != TypeAuto && !isIntegerType(t.Kind) {
				a.problem(e.Operator.Pos, "invalid operand of type %q to bitwise NOT", t.String())
			}
			return t
		case token.ASTERISK:
			if t.Kind == TypePointer && t.Base != nil {
				return *t.Base
			}
			if t.Kind == TypeArray && t.Base != nil {
				return *t.Base
			}
			if t.Kind == TypeAuto {
				return Type{Kind: TypeAuto}
			}
			if t.Kind != TypeUnknown {
				a.problem(e.Operator.Pos, "invalid operand of type %q to unary '*'", t.String())
			}
			return Type{Kind: TypeUnknown}
		case token.BIT_AND:
			if !assignable(e.Operand) && !isFunctionOrCompoundLiteral(e.Operand) {
				a.problem(e.Operator.Pos, "cannot take address of non-lvalue")
			}
			return Type{Kind: TypePointer, Base: &t}
		case token.INCREMENT, token.DECREMENT:
			if !assignable(e.Operand) {
				a.problem(e.Operator.Pos, "lvalue required as %s operand", e.Operator.Literal)
			} else if t.IsConst {
				a.problem(e.Operator.Pos, "cannot modify read-only value")
			} else if t.Kind != TypeUnknown && t.Kind != TypeAuto && !isScalarType(t.Kind) {
				a.problem(e.Operator.Pos, "wrong type argument to %s", e.Operator.Literal)
			}
			return t
		}
		return t

	case *parser.BinaryExpr:
		lt := a.expression(e.Left)
		rt := a.expression(e.Right)

		switch e.Operator.Type {
		case token.PERCENT, token.BIT_AND, token.BIT_OR, token.BIT_XOR, token.SHL, token.SHR:
			if (lt.Kind != TypeUnknown && lt.Kind != TypeAuto && !isIntegerType(lt.Kind)) || (rt.Kind != TypeUnknown && rt.Kind != TypeAuto && !isIntegerType(rt.Kind)) {
				a.problem(e.Operator.Pos, "invalid operands to binary %s (have %q and %q)", e.Operator.Literal, lt.String(), rt.String())
			}
			return Type{Kind: TypeInt}
		case token.LOGICAL_AND, token.LOGICAL_OR:
			return Type{Kind: TypeBool}
		case token.EQ, token.NOT_EQ, token.LT, token.LTE, token.GT, token.GTE:
			if lt.Kind != TypeUnknown && rt.Kind != TypeUnknown && !a.isCompatible(lt, rt) {
				a.problem(e.Operator.Pos, "comparison between incompatible types (%q and %q)", lt.String(), rt.String())
			}
			return Type{Kind: TypeBool}
		case token.PLUS:
			if lt.Kind == TypeString || rt.Kind == TypeString {
				return Type{Kind: TypeString}
			}
			if lt.Kind == TypePointer {
				return lt
			}
			if rt.Kind == TypePointer {
				return rt
			}
			if lt.Kind == TypeFloat || rt.Kind == TypeFloat {
				return Type{Kind: TypeFloat}
			}
			return Type{Kind: TypeInt}
		case token.MINUS:
			if lt.Kind == TypePointer {
				return lt
			}
			if lt.Kind == TypeFloat || rt.Kind == TypeFloat {
				return Type{Kind: TypeFloat}
			}
			return Type{Kind: TypeInt}
		default:
			if lt.Kind == TypeFloat || rt.Kind == TypeFloat {
				return Type{Kind: TypeFloat}
			}
			return Type{Kind: TypeInt}
		}

	case *parser.AssignExpr:
		if !assignable(e.Left) {
			a.problem(e.Left.Position(), "left operand of %s is not assignable", e.Operator.Literal)
		}
		lt := a.expression(e.Left)
		rt := a.expression(e.Right)

		if lt.IsConst {
			a.problem(e.Operator.Pos, "cannot assign to read-only target")
		}

		if e.Operator.Type == token.PLUS_ASSIGN && lt.Kind == TypeString && (rt.Kind == TypeString || rt.Kind == TypeChar) {
			return lt
		}

		if isCompoundBitwiseOperator(e.Operator.Type) {
			if (lt.Kind != TypeUnknown && lt.Kind != TypeAuto && !isIntegerType(lt.Kind)) || (rt.Kind != TypeUnknown && rt.Kind != TypeAuto && !isIntegerType(rt.Kind)) {
				a.problem(e.Operator.Pos, "invalid operands to binary %s (have %q and %q)", e.Operator.Literal, lt.String(), rt.String())
			}
		} else if lt.Kind != TypeUnknown && rt.Kind != TypeUnknown {
			if !a.isCompatible(lt, rt) {
				a.problem(e.Operator.Pos, "incompatible type in assignment (assigning %q to %q)", rt.String(), lt.String())
			}
		}
		return lt

	case *parser.ConditionalExpr:
		a.expression(e.Condition)
		tt := a.expression(e.Then)
		et := a.expression(e.Else)
		if tt.Kind != TypeUnknown && et.Kind != TypeUnknown && !a.isCompatible(tt, et) {
			a.problem(e.Question.Pos, "incompatible types in conditional expression (%q and %q)", tt.String(), et.String())
		}
		return tt

	case *parser.CallExpr:
		ft := a.expression(e.Function)
		argTypes := make([]Type, len(e.Arguments))
		for i, argument := range e.Arguments {
			argTypes[i] = a.expression(argument)
		}

		funcType, isFunc := unwrapFunc(ft)
		if isFunc {
			ft = funcType
		}

		if ft.Kind == TypeFunction {
			if !ft.Variadic && len(e.Arguments) != len(ft.Params) {
				a.problem(e.Open.Pos, "wrong number of arguments to function call: expected %d, got %d", len(ft.Params), len(e.Arguments))
			} else if ft.Variadic && len(e.Arguments) < len(ft.Params) {
				a.problem(e.Open.Pos, "too few arguments to function call: expected at least %d, got %d", len(ft.Params), len(e.Arguments))
			} else {
				for i := 0; i < len(ft.Params) && i < len(argTypes); i++ {
					if argTypes[i].Kind != TypeUnknown && !a.isCompatible(ft.Params[i], argTypes[i]) {
						a.problem(e.Arguments[i].Position(), "incompatible type for argument %d in function call (expected %q, got %q)", i+1, ft.Params[i].String(), argTypes[i].String())
					}
				}
			}
			if ft.ReturnType != nil {
				return *ft.ReturnType
			}
			return Type{Kind: TypeVoid}
		} else if ft.Kind == TypeAuto {
			return Type{Kind: TypeAuto}
		} else if ft.Kind != TypeUnknown {
			a.problem(e.Open.Pos, "called object of type %q is not a function", ft.String())
		}
		return Type{Kind: TypeUnknown}

	case *parser.IndexExpr:
		vt := a.expression(e.Value)
		it := a.expression(e.Index)

		if vt.Kind != TypeUnknown && vt.Kind != TypeAuto && vt.Kind != TypeArray && vt.Kind != TypePointer && vt.Kind != TypeString {
			a.problem(e.Open.Pos, "subscripted value is not an array or pointer (has type %q)", vt.String())
		}
		if it.Kind != TypeUnknown && it.Kind != TypeAuto && !isIntegerType(it.Kind) {
			a.problem(e.Index.Position(), "array subscript is not an integer (has type %q)", it.String())
		}

		if vt.Kind == TypeArray || vt.Kind == TypePointer {
			if vt.Base != nil {
				res := *vt.Base
				if vt.IsConst {
					res.IsConst = true
				}
				return res
			}
		} else if vt.Kind == TypeString {
			return Type{Kind: TypeChar}
		} else if vt.Kind == TypeAuto {
			return Type{Kind: TypeAuto}
		}
		return Type{Kind: TypeUnknown}

	case *parser.MemberExpr:
		vt := a.expression(e.Value)
		var st Type

		if e.Operator.Type == token.DOT {
			if vt.Kind == TypePointer && vt.Base != nil && (vt.Base.Kind == TypeStruct || vt.Base.Kind == TypeUnion) {
				a.problem(e.Operator.Pos, "member reference type %q is a pointer; did you mean '->'?", vt.String())
				st = *vt.Base
			} else if vt.Kind == TypeStruct || vt.Kind == TypeUnion {
				st = vt
			} else if vt.Kind == TypeAuto {
				return Type{Kind: TypeAuto}
			} else if vt.Kind != TypeUnknown {
				a.problem(e.Operator.Pos, "expected struct or union before '.' operator (has type %q)", vt.String())
				return Type{Kind: TypeUnknown}
			}
		} else if e.Operator.Type == token.ARROW {
			if vt.Kind == TypeStruct || vt.Kind == TypeUnion {
				a.problem(e.Operator.Pos, "member reference type %q is not a pointer; did you mean '.'?", vt.String())
				st = vt
			} else if vt.Kind == TypePointer && vt.Base != nil && (vt.Base.Kind == TypeStruct || vt.Base.Kind == TypeUnion) {
				st = *vt.Base
			} else if vt.Kind == TypeAuto {
				return Type{Kind: TypeAuto}
			} else if vt.Kind != TypeUnknown {
				a.problem(e.Operator.Pos, "expected pointer to struct or union before '->' operator (has type %q)", vt.String())
				return Type{Kind: TypeUnknown}
			}
		}

		if st.Kind == TypeStruct || st.Kind == TypeUnion {
			for _, m := range st.Members {
				if m.Name == e.Member.Literal {
					res := m.Type
					if st.IsConst || vt.IsConst || m.IsConst {
						res.IsConst = true
					}
					return res
				}
			}
			a.problem(e.Member.Pos, "%q is not a member of %s", e.Member.Literal, st.String())
			return Type{Kind: TypeUnknown}
		}
		return Type{Kind: TypeUnknown}

	case *parser.CastExpr:
		specsType := a.typeFromSpecs(e.Type)
		castType := a.typeFromDeclarator(specsType, e.Declarator)
		vType := a.expression(e.Value)
		if vType.Kind != TypeUnknown && vType.Kind != TypeAuto && castType.Kind != TypeUnknown && castType.Kind != TypeAuto {
			if !a.isCompatible(castType, vType) {
				a.problem(e.Open.Pos, "type conversion error: cannot cast %s to %s", vType.String(), castType.String())
			}
		}
		return castType

	case *parser.SizeofExpr:
		if e.Value != nil {
			a.expression(e.Value)
		}
		if len(e.Type) > 0 {
			t := a.typeFromSpecs(e.Type)
			if e.Declarator.Name.Literal != "" || len(e.Declarator.Pointers) > 0 || len(e.Declarator.Suffixes) > 0 {
				a.typeFromDeclarator(t, e.Declarator)
			}
		}
		return Type{Kind: TypeInt}

	case *parser.CommaExpr:
		var last Type
		for _, x := range e.Expressions {
			last = a.expression(x)
		}
		return last

	case *parser.CompoundLiteralExpr:
		specsType := a.typeFromSpecs(e.Type)
		clType := a.typeFromDeclarator(specsType, e.Declarator)
		a.expression(e.Initializer)
		return clType

	case *parser.InitializerListExpr:
		for _, value := range e.Values {
			for _, designator := range value.Designators {
				a.expression(designator.Index)
			}
			a.expression(value.Value)
		}
		return Type{Kind: TypeUnknown}
	}
	return Type{Kind: TypeUnknown}
}

func unwrapFunc(t Type) (Type, bool) {
	for t.Kind == TypePointer && t.Base != nil {
		t = *t.Base
	}
	if t.Kind == TypeFunction {
		return t, true
	}
	return Type{}, false
}

func (a *Analyzer) isFuncCompatible(target, source Type) bool {
	if target.ReturnType != nil && source.ReturnType != nil {
		if !a.isCompatible(*target.ReturnType, *source.ReturnType) {
			return false
		}
	} else if (target.ReturnType == nil) != (source.ReturnType == nil) {
		return false
	}

	if target.Variadic != source.Variadic {
		return false
	}

	if len(target.Params) != len(source.Params) {
		return false
	}

	for i := 0; i < len(target.Params); i++ {
		if !a.isCompatible(target.Params[i], source.Params[i]) {
			return false
		}
	}

	return true
}

func (a *Analyzer) isCompatible(target, source Type) bool {
	if target.Kind == TypeUnknown || source.Kind == TypeUnknown || target.Kind == TypeAuto || source.Kind == TypeAuto {
		return true
	}

	targetFunc, isTargetFunc := unwrapFunc(target)
	sourceFunc, isSourceFunc := unwrapFunc(source)

	if isTargetFunc || isSourceFunc {
		if isTargetFunc && isSourceFunc {
			return a.isFuncCompatible(targetFunc, sourceFunc)
		}
		if isTargetFunc {
			if isZeroLiteral(source) {
				return true
			}
			if source.Kind == TypePointer && source.Base != nil && source.Base.Kind == TypeVoid {
				return true
			}
			return false
		}
		if isSourceFunc {
			if target.Kind == TypePointer && target.Base != nil && target.Base.Kind == TypeVoid {
				return true
			}
			return false
		}
	}

	if target.Kind == source.Kind {
		switch target.Kind {
		case TypeStruct, TypeUnion:
			if target.Tag != "" && source.Tag != "" {
				return target.Tag == source.Tag
			}
			return true
		case TypePointer:
			if target.Base == nil || source.Base == nil {
				return true
			}
			if target.Base.Kind == TypeVoid || source.Base.Kind == TypeVoid {
				return true
			}
			return a.isCompatible(*target.Base, *source.Base)
		case TypeArray:
			if target.Base == nil || source.Base == nil {
				return true
			}
			return a.isCompatible(*target.Base, *source.Base)
		default:
			return true
		}
	}

	if isNumericType(target.Kind) && isNumericType(source.Kind) {
		return true
	}

	if (target.Kind == TypeStruct || target.Kind == TypeArray) && isZeroLiteral(source) {
		return true
	}

	if target.Kind == TypePointer && isIntegerType(source.Kind) {
		return true
	}
	if source.Kind == TypePointer && isIntegerType(target.Kind) {
		return true
	}

	if target.Kind == TypePointer && source.Kind == TypeArray {
		if target.Base == nil || source.Base == nil {
			return true
		}
		if target.Base.Kind == TypeVoid {
			return true
		}
		return a.isCompatible(*target.Base, *source.Base)
	}

	if target.Kind == TypeString {
		if source.Kind == TypeChar || (source.Kind == TypeArray && source.Base != nil && source.Base.Kind == TypeChar) || (source.Kind == TypePointer && source.Base != nil && source.Base.Kind == TypeChar) {
			return true
		}
	}

	return false
}

func isZeroLiteral(t Type) bool {
	return t.Kind == TypeInt
}

func isNumericType(k TypeKind) bool {
	return k == TypeInt || k == TypeFloat || k == TypeChar || k == TypeBool
}

func isIntegerType(k TypeKind) bool {
	return k == TypeInt || k == TypeChar || k == TypeBool
}

func isScalarType(k TypeKind) bool {
	return isNumericType(k) || k == TypePointer || k == TypeString
}

func isCompoundBitwiseOperator(op token.TokenType) bool {
	return op == token.PERCENT_ASSIGN || op == token.BIT_AND_ASSIGN || op == token.BIT_OR_ASSIGN || op == token.BIT_XOR_ASSIGN || op == token.SHL_ASSIGN || op == token.SHR_ASSIGN
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

func isFunctionOrCompoundLiteral(expression parser.Expression) bool {
	switch expression.(type) {
	case *parser.IdentExpr, *parser.CompoundLiteralExpr:
		return true
	}
	return false
}
