package parser

import (
	"fmt"
	"tvshow/lang/token"
)

// Parser parses a preprocessed token stream. Tokens may include one EOF token.
type Parser struct {
	tokens   []token.Token
	i        int
	typedefs map[string]bool
}

func New(tokens []token.Token) *Parser { return &Parser{tokens: tokens, typedefs: map[string]bool{}} }

// Parse parses tokens into a translation unit.
func Parse(tokens []token.Token) (*Program, error) { return New(tokens).ParseProgram() }
func (p *Parser) ParseProgram() (*Program, error) {
	n := &Program{}
	for p.cur().Type != token.EOF {
		d, e := p.parseExternal()
		if e != nil {
			return nil, e
		}
		n.Declarations = append(n.Declarations, d)
	}
	n.End = p.cur()
	return n, nil
}
func (p *Parser) cur() token.Token {
	if p.i >= len(p.tokens) {
		return token.Token{Type: token.EOF}
	}
	return p.tokens[p.i]
}
func (p *Parser) next() token.Token {
	t := p.cur()
	if p.i < len(p.tokens) {
		p.i++
	}
	return t
}
func (p *Parser) accept(k token.TokenType) (token.Token, bool) {
	if p.cur().Type == k {
		return p.next(), true
	}
	return token.Token{}, false
}
func (p *Parser) expect(k token.TokenType) (token.Token, error) {
	if t, ok := p.accept(k); ok {
		return t, nil
	}
	return token.Token{}, p.err("expected %s, got %s", k, p.cur())
}
func (p *Parser) err(f string, a ...any) error {
	return fmt.Errorf("%s: %s", p.cur().Pos.String(), fmt.Sprintf(f, a...))
}

func (p *Parser) parseExternal() (Declaration, error) {
	specs, e := p.parseSpecs()
	if e != nil {
		return nil, e
	}
	ds, e := p.parseDeclarator()
	if e != nil {
		return nil, e
	}
	if p.cur().Type == token.LBRACE {
		b, e := p.parseBlock()
		return &FunctionDecl{specs, ds, b}, e
	}
	decls := []Declarator{ds}
	for {
		if _, ok := p.accept(token.COMMA); !ok {
			break
		}
		d, e := p.parseDeclarator()
		if e != nil {
			return nil, e
		}
		decls = append(decls, d)
	}
	semi, e := p.expect(token.SEMICOLON)
	if e != nil {
		return nil, e
	}
	n := &VarDecl{Specs: specs, Declarators: decls, Semi: semi}
	if has(specs, token.TYPEDEF) {
		for _, d := range decls {
			p.typedefs[d.Name.Literal] = true
		}
	}
	return n, nil
}
func has(s []TypeSpec, k token.TokenType) bool {
	for _, x := range s {
		if x.Token.Type == k {
			return true
		}
	}
	return false
}
func (p *Parser) parseSpecs() ([]TypeSpec, error) {
	var out []TypeSpec
	for {
		t := p.cur()
		if isQualifier(t.Type) || isStorage(t.Type) || isBuiltin(t.Type) || t.Type == token.STRUCT || t.Type == token.UNION || t.Type == token.ENUM || (t.Type == token.IDENT && p.typedefs[t.Literal]) {
			p.next()
			s := TypeSpec{Token: t}
			if t.Type == token.STRUCT || t.Type == token.UNION || t.Type == token.ENUM {
				if p.cur().Type == token.IDENT {
					s.Tag = p.next().Literal
				}
				if _, ok := p.accept(token.LBRACE); ok {
					if t.Type == token.ENUM {
						for p.cur().Type != token.RBRACE {
							name, e := p.expect(token.IDENT)
							if e != nil {
								return nil, e
							}
							v := Enumerator{Name: name}
							if _, ok := p.accept(token.ASSIGN); ok {
								v.Value, e = p.expr(1)
								if e != nil {
									return nil, e
								}
							}
							s.EnumValues = append(s.EnumValues, v)
							if _, ok := p.accept(token.COMMA); !ok {
								break
							}
						}
					} else {
						for p.cur().Type != token.RBRACE {
							d, e := p.parseDeclaration()
							if e != nil {
								return nil, e
							}
							s.Members = append(s.Members, d)
						}
					}
					if _, e := p.expect(token.RBRACE); e != nil {
						return nil, e
					}
				}
			}
			out = append(out, s)
			continue
		}
		break
	}
	if len(out) == 0 {
		return nil, p.err("expected declaration specifier")
	}
	return out, nil
}
func isQualifier(k token.TokenType) bool {
	return k == token.CONST
}
func isStorage(k token.TokenType) bool {
	return k == token.TYPEDEF || k == token.AUTO
}
func isBuiltin(k token.TokenType) bool {
	return k == token.AUTO || k == token.VOID || k == token.CHAR_KW || k == token.SHORT || k == token.INT_KW || k == token.LONG || k == token.FLOAT_KW || k == token.DOUBLE || k == token.SIGNED || k == token.UNSIGNED || k == token.BOOL || k == token.COMPLEX || k == token.IMAGINARY || k == token.STRING_KW
}
func (p *Parser) parseDeclarator() (Declarator, error) {
	var d Declarator
	for p.cur().Type == token.ASTERISK {
		q := Pointer{Token: p.next()}
		for isQualifier(p.cur().Type) {
			q.Qualifiers = append(q.Qualifiers, p.next())
		}
		d.Pointers = append(d.Pointers, q)
	}
	if p.cur().Type == token.IDENT {
		d.Name = p.next()
	} else if _, ok := p.accept(token.LPAREN); ok {
		x, e := p.parseDeclarator()
		if e != nil {
			return d, e
		}
		d.Name = x.Name
		d.FuncPointers = x.Pointers
		if _, e = p.expect(token.RPAREN); e != nil {
			return d, e
		}
	}
	for p.cur().Type == token.LPAREN || p.cur().Type == token.LBRACK {
		if p.cur().Type == token.LPAREN {
			o := p.next()
			f := &FunctionSuffix{Open: o}
			if p.cur().Type != token.RPAREN {
				for {
					if _, ok := p.accept(token.ELLIPSIS); ok {
						f.Variadic = true
						break
					}
					ss, e := p.parseSpecs()
					if e != nil {
						return d, e
					}
					pd, e := p.parseDeclarator()
					if e != nil {
						return d, e
					}
					f.Parameters = append(f.Parameters, Parameter{ss, pd})
					if _, ok := p.accept(token.COMMA); !ok {
						break
					}
				}
			}
			if _, e := p.expect(token.RPAREN); e != nil {
				return d, e
			}
			d.Suffixes = append(d.Suffixes, f)
		} else {
			o := p.next()
			a := &ArraySuffix{Open: o}
			for isQualifier(p.cur().Type) {
				a.Qualifiers = append(a.Qualifiers, p.next())
			}
			if p.cur().Type != token.RBRACK {
				x, e := p.expr(1)
				if e != nil {
					return d, e
				}
				a.Size = x
			}
			if _, e := p.expect(token.RBRACK); e != nil {
				return d, e
			}
			d.Suffixes = append(d.Suffixes, a)
		}
	}
	if _, ok := p.accept(token.ASSIGN); ok {
		x, e := p.parseInitializer()
		if e != nil {
			return d, e
		}
		d.Initializer = x
	}
	return d, nil
}
func (p *Parser) parseDeclaration() (Declaration, error) {
	s, e := p.parseSpecs()
	if e != nil {
		return nil, e
	}
	d, e := p.parseDeclarator()
	if e != nil {
		return nil, e
	}
	ds := []Declarator{d}
	for {
		if _, ok := p.accept(token.COMMA); !ok {
			break
		}
		x, e := p.parseDeclarator()
		if e != nil {
			return nil, e
		}
		ds = append(ds, x)
	}
	semi, e := p.expect(token.SEMICOLON)
	if e != nil {
		return nil, e
	}
	n := &VarDecl{s, ds, semi}
	if has(s, token.TYPEDEF) {
		for _, x := range ds {
			p.typedefs[x.Name.Literal] = true
		}
	}
	return n, nil
}
func (p *Parser) parseBlock() (*BlockStmt, error) {
	o, e := p.expect(token.LBRACE)
	if e != nil {
		return nil, e
	}
	b := &BlockStmt{Open: o}
	for p.cur().Type != token.RBRACE {
		if p.cur().Type == token.EOF {
			return nil, p.err("unterminated block")
		}
		if p.startsDeclaration() {
			d, e := p.parseDeclaration()
			if e != nil {
				return nil, e
			}
			b.Items = append(b.Items, d)
		} else {
			s, e := p.parseStmt()
			if e != nil {
				return nil, e
			}
			b.Items = append(b.Items, s)
		}
	}
	b.Close, e = p.expect(token.RBRACE)
	return b, e
}
func (p *Parser) startsTypeToken(t token.Token) bool {
	return isQualifier(t.Type) || isStorage(t.Type) || isBuiltin(t.Type) || t.Type == token.STRUCT || t.Type == token.UNION || t.Type == token.ENUM || (t.Type == token.IDENT && p.typedefs[t.Literal])
}
func (p *Parser) startsDeclaration() bool {
	return p.startsTypeToken(p.cur())
}
func (p *Parser) parseStmt() (Statement, error) {
	t := p.cur()
	switch t.Type {
	case token.LBRACE:
		return p.parseBlock()
	case token.IF:
		p.next()
		if _, e := p.expect(token.LPAREN); e != nil {
			return nil, e
		}
		c, e := p.expr(1)
		if e != nil {
			return nil, e
		}
		if _, e = p.expect(token.RPAREN); e != nil {
			return nil, e
		}
		th, e := p.parseStmt()
		if e != nil {
			return nil, e
		}
		n := &IfStmt{Token: t, Condition: c, Then: th}
		if _, ok := p.accept(token.ELSE); ok {
			n.Else, e = p.parseStmt()
		}
		return n, e
	case token.WHILE:
		p.next()
		_, e := p.expect(token.LPAREN)
		if e != nil {
			return nil, e
		}
		c, e := p.expr(1)
		if e != nil {
			return nil, e
		}
		_, e = p.expect(token.RPAREN)
		if e != nil {
			return nil, e
		}
		b, e := p.parseStmt()
		return &WhileStmt{t, c, b}, e
	case token.DO:
		p.next()
		b, e := p.parseStmt()
		if e != nil {
			return nil, e
		}
		if _, e = p.expect(token.WHILE); e != nil {
			return nil, e
		}
		_, e = p.expect(token.LPAREN)
		if e != nil {
			return nil, e
		}
		c, e := p.expr(1)
		if e != nil {
			return nil, e
		}
		_, e = p.expect(token.RPAREN)
		if e != nil {
			return nil, e
		}
		_, e = p.expect(token.SEMICOLON)
		return &DoWhileStmt{t, b, c}, e
	case token.FOR:
		return p.parseFor()
	case token.SWITCH:
		p.next()
		_, e := p.expect(token.LPAREN)
		if e != nil {
			return nil, e
		}
		v, e := p.expr(1)
		if e != nil {
			return nil, e
		}
		_, e = p.expect(token.RPAREN)
		if e != nil {
			return nil, e
		}
		b, e := p.parseStmt()
		return &SwitchStmt{t, v, b}, e
	case token.CASE:
		p.next()
		v, e := p.expr(1)
		if e != nil {
			return nil, e
		}
		_, e = p.expect(token.COLON)
		if e != nil {
			return nil, e
		}
		b, e := p.parseStmt()
		return &CaseStmt{t, v, b}, e
	case token.DEFAULT:
		p.next()
		_, e := p.expect(token.COLON)
		if e != nil {
			return nil, e
		}
		b, e := p.parseStmt()
		return &CaseStmt{Token: t, Body: b}, e
	case token.BREAK, token.CONTINUE, token.RETURN, token.GOTO:
		p.next()
		n := &JumpStmt{Token: t}
		var e error
		if t.Type == token.GOTO {
			n.Label, e = p.expect(token.IDENT)
		} else if t.Type == token.RETURN && p.cur().Type != token.SEMICOLON {
			n.Value, e = p.expr(1)
		}
		if e != nil {
			return nil, e
		}
		n.Semi, e = p.expect(token.SEMICOLON)
		return n, e
	case token.THROW:
		p.next()
		val, e := p.expr(1)
		if e != nil {
			return nil, e
		}
		semi, e := p.expect(token.SEMICOLON)
		if e != nil {
			return nil, e
		}
		return &ThrowStmt{Token: t, Value: val, Semi: semi}, nil
	case token.TRY:
		p.next()
		body, e := p.parseBlock()
		if e != nil {
			return nil, e
		}
		catchTok, e := p.expect(token.CATCH)
		if e != nil {
			return nil, e
		}
		if _, e = p.expect(token.LPAREN); e != nil {
			return nil, e
		}
		specs, e := p.parseSpecs()
		if e != nil {
			return nil, e
		}
		decl, e := p.parseDeclarator()
		if e != nil {
			return nil, e
		}
		if _, e = p.expect(token.RPAREN); e != nil {
			return nil, e
		}
		catchBody, e := p.parseBlock()
		if e != nil {
			return nil, e
		}
		return &TryCatchStmt{
			Token: t,
			Body:  body,
			Catch: &CatchBlock{
				Token:      catchTok,
				VarType:    specs,
				Declarator: decl,
				Body:       catchBody,
			},
		}, nil
	}
	if t.Type == token.IDENT && p.i+1 < len(p.tokens) && p.tokens[p.i+1].Type == token.COLON {
		p.next()
		c, _ := p.expect(token.COLON)
		s, e := p.parseStmt()
		return &LabelStmt{t, c, s}, e
	}
	semi := token.Token{}
	if p.cur().Type == token.SEMICOLON {
		semi = p.next()
		return &ExprStmt{Semi: semi}, nil
	}
	x, e := p.expr(1)
	if e == nil {
		semi, e = p.expect(token.SEMICOLON)
	}
	return &ExprStmt{x, semi}, e
}

func (p *Parser) parseFor() (Statement, error) {
	t := p.next()
	if _, e := p.expect(token.LPAREN); e != nil {
		return nil, e
	}
	n := &ForStmt{Token: t}
	var e error
	if p.startsDeclaration() {
		n.Init, e = p.parseDeclaration()
	} else if p.cur().Type == token.SEMICOLON {
		p.next()
	} else {
		x, er := p.expr(1)
		e = er
		if e == nil {
			_, e = p.expect(token.SEMICOLON)
		}
		n.Init = &ExprStmt{Expr: x}
	}
	if e != nil {
		return nil, e
	}
	if p.cur().Type != token.SEMICOLON {
		n.Condition, e = p.expr(1)
		if e != nil {
			return nil, e
		}
	}
	if _, e = p.expect(token.SEMICOLON); e != nil {
		return nil, e
	}
	if p.cur().Type != token.RPAREN {
		n.Post, e = p.expr(1)
		if e != nil {
			return nil, e
		}
	}
	if _, e = p.expect(token.RPAREN); e != nil {
		return nil, e
	}
	n.Body, e = p.parseStmt()
	return n, e
}
func (p *Parser) parseInitializer() (Expression, error) {
	if p.cur().Type != token.LBRACE {
		return p.expr(2)
	}
	o := p.next()
	n := &InitializerListExpr{Open: o}
	for p.cur().Type != token.RBRACE {
		var designators []Designator
		for p.cur().Type == token.DOT || p.cur().Type == token.LBRACK {
			dTok := p.next()
			d := Designator{Token: dTok}
			if dTok.Type == token.DOT {
				fTok, e := p.expect(token.IDENT)
				if e != nil {
					return nil, e
				}
				d.Field = fTok
			} else {
				iExpr, e := p.expr(1)
				if e != nil {
					return nil, e
				}
				if _, e := p.expect(token.RBRACK); e != nil {
					return nil, e
				}
				d.Index = iExpr
			}
			designators = append(designators, d)
		}
		if len(designators) > 0 {
			p.accept(token.ASSIGN)
		}
		v, e := p.parseInitializer()
		if e != nil {
			return nil, e
		}
		n.Values = append(n.Values, InitializerElement{Designators: designators, Value: v})
		if _, ok := p.accept(token.COMMA); !ok {
			break
		}
	}
	_, e := p.expect(token.RBRACE)
	return n, e
}

var precedence = map[token.TokenType]int{token.COMMA: 1, token.ASSIGN: 2, token.PLUS_ASSIGN: 2, token.MINUS_ASSIGN: 2, token.ASTERISK_ASSIGN: 2, token.SLASH_ASSIGN: 2, token.PERCENT_ASSIGN: 2, token.LOGICAL_OR: 4, token.LOGICAL_AND: 5, token.BIT_OR: 6, token.BIT_XOR: 7, token.BIT_AND: 8, token.EQ: 9, token.NOT_EQ: 9, token.LT: 10, token.LTE: 10, token.GT: 10, token.GTE: 10, token.SHL: 11, token.SHR: 11, token.PLUS: 12, token.MINUS: 12, token.ASTERISK: 13, token.SLASH: 13, token.PERCENT: 13}

func (p *Parser) expr(min int) (Expression, error) {
	left, e := p.prefix()
	if e != nil {
		return nil, e
	}
	for {
		if p.cur().Type == token.QUESTION && min <= 3 {
			question := p.next()
			then, e := p.expr(1)
			if e != nil {
				return nil, e
			}
			if _, e = p.expect(token.COLON); e != nil {
				return nil, e
			}
			otherwise, e := p.expr(3)
			if e != nil {
				return nil, e
			}
			left = &ConditionalExpr{Condition: left, Question: question, Then: then, Else: otherwise}
			continue
		}
		op := p.cur()
		prec := precedence[op.Type]
		if prec < min || prec == 0 {
			break
		}
		p.next()
		rm := prec + 1
		if prec == 2 {
			rm = prec
		}
		r, e := p.expr(rm)
		if e != nil {
			return nil, e
		}
		if prec == 2 {
			left = &AssignExpr{left, op, r}
		} else if op.Type == token.COMMA {
			left = &CommaExpr{[]Expression{left, r}}
		} else {
			left = &BinaryExpr{left, op, r}
		}
	}
	return left, nil
}
func (p *Parser) prefix() (Expression, error) {
	t := p.next()
	var x Expression
	switch t.Type {
	case token.IDENT:
		x = &IdentExpr{t}
	case token.INT, token.FLOAT, token.CHAR, token.STRING:
		x = &LiteralExpr{t}
	case token.SIZEOF:
		if p.cur().Type == token.LPAREN && p.i+1 < len(p.tokens) && p.startsTypeToken(p.tokens[p.i+1]) {
			p.next()
			specs, e := p.parseSpecs()
			if e != nil {
				return nil, e
			}
			decl, e := p.parseDeclarator()
			if e != nil {
				return nil, e
			}
			if _, e = p.expect(token.RPAREN); e != nil {
				return nil, e
			}
			x = &SizeofExpr{Token: t, Type: specs, Declarator: decl}
		} else {
			val, e := p.expr(13)
			if e != nil {
				return nil, e
			}
			x = &SizeofExpr{Token: t, Value: val}
		}
	case token.LPAREN:
		if p.startsDeclaration() {
			specs, e := p.parseSpecs()
			if e != nil {
				return nil, e
			}
			decl, e := p.parseDeclarator()
			if e != nil {
				return nil, e
			}
			if _, e = p.expect(token.RPAREN); e != nil {
				return nil, e
			}
			if p.cur().Type == token.LBRACE {
				init, e := p.parseInitializer()
				if e != nil {
					return nil, e
				}
				x = &CompoundLiteralExpr{Open: t, Type: specs, Declarator: decl, Initializer: init}
			} else {
				val, e := p.expr(13)
				if e != nil {
					return nil, e
				}
				x = &CastExpr{Open: t, Type: specs, Declarator: decl, Value: val}
			}
		} else {
			var e error
			x, e = p.expr(1)
			if e != nil {
				return nil, e
			}
			if _, e = p.expect(token.RPAREN); e != nil {
				return nil, e
			}
		}
	case token.PLUS, token.MINUS, token.LOGICAL_NOT, token.BIT_NOT, token.ASTERISK, token.BIT_AND, token.INCREMENT, token.DECREMENT:
		v, e := p.prefix()
		if e != nil {
			return nil, e
		}
		x = &UnaryExpr{Operator: t, Operand: v}
	default:
		return nil, fmt.Errorf("%s: expected expression, got %s", t.Pos.String(), t)
	}
	for {
		op := p.cur()
		if op.Type == token.LPAREN {
			p.next()
			c := &CallExpr{Function: x, Open: op}
			if p.cur().Type != token.RPAREN {
				for {
					a, e := p.expr(2)
					if e != nil {
						return nil, e
					}
					c.Arguments = append(c.Arguments, a)
					if _, ok := p.accept(token.COMMA); !ok {
						break
					}
				}
			}
			if _, e := p.expect(token.RPAREN); e != nil {
				return nil, e
			}
			x = c
			continue
		}
		if op.Type == token.LBRACK {
			p.next()
			i, e := p.expr(1)
			if e != nil {
				return nil, e
			}
			if _, e = p.expect(token.RBRACK); e != nil {
				return nil, e
			}
			x = &IndexExpr{x, op, i}
			continue
		}
		if op.Type == token.DOT || op.Type == token.ARROW {
			p.next()
			m, e := p.expect(token.IDENT)
			if e != nil {
				return nil, e
			}
			x = &MemberExpr{x, op, m}
			continue
		}
		if op.Type == token.INCREMENT || op.Type == token.DECREMENT {
			p.next()
			x = &UnaryExpr{Operator: op, Operand: x, Postfix: true}
			continue
		}
		break
	}
	return x, nil
}
