// Package macro implements the Scintilla preprocessor's token based macro expander.
package macro

import (
	"fmt"
	"strconv"
	"strings"

	"tvshow/lang/lexer"
	"tvshow/lang/token"
)

// Macro is a preprocessor macro. Parameters being nil denotes an object-like
// macro; an empty, non-nil Parameters slice denotes a function-like macro with
// no parameters.
type Macro struct {
	Name        string
	Parameters  []string
	Variadic    bool
	Replacement []token.Token
}

// FunctionLike reports whether m requires an argument list to expand.
func (m Macro) FunctionLike() bool { return m.Parameters != nil }

// IncludeResolver supplies tokens for an #include operand. The operand is the
// unquoted include name, and position is that of the directive.
type IncludeResolver func(name string, pos token.Position) ([]token.Token, error)

// Expander expands C99-style preprocessor directives and macros.
type Expander struct {
	Macros  map[string]Macro
	Include IncludeResolver
	// MaxExpansion limits recursive replacement work. Zero uses a conservative
	// default, preventing malformed self-referential inputs from looping forever.
	MaxExpansion int
}

// New returns an empty macro expander.
func New() *Expander { return &Expander{Macros: make(map[string]Macro), MaxExpansion: 10000} }

// Define installs or replaces a macro. Replacement tokens are copied so callers
// may safely reuse their input slice.
func (e *Expander) Define(m Macro) {
	if e.Macros == nil {
		e.Macros = make(map[string]Macro)
	}
	m.Parameters = append([]string(nil), m.Parameters...)
	m.Replacement = withoutEOF(append([]token.Token(nil), m.Replacement...))
	e.Macros[m.Name] = m
}

// Undef removes a macro definition.
func (e *Expander) Undef(name string) { delete(e.Macros, name) }

// Expand processes directives and returns the expanded token stream. EOF tokens
// in the input are ignored and exactly one EOF token is returned.
func (e *Expander) Expand(input []token.Token) ([]token.Token, error) {
	if e.Macros == nil {
		e.Macros = make(map[string]Macro)
	}
	limit := e.MaxExpansion
	if limit <= 0 {
		limit = 10000
	}
	out, err := e.directives(withoutEOF(input), limit)
	if err != nil {
		return nil, err
	}
	out, err = e.expand(out, map[string]bool{}, &limit)
	if err != nil {
		return nil, err
	}
	pos := token.Position{}
	if len(input) > 0 {
		pos = input[len(input)-1].Pos
	}
	return append(out, token.Token{Type: token.EOF, Pos: pos}), nil
}

// Expand expands tokens with a new empty expander.
func Expand(input []token.Token) ([]token.Token, error) { return New().Expand(input) }

type condition struct{ parent, active, taken bool }

func (e *Expander) directives(in []token.Token, limit int) ([]token.Token, error) {
	var out []token.Token
	var stack []condition
	enabled := func() bool { return len(stack) == 0 || stack[len(stack)-1].active }
	for i := 0; i < len(in); {
		if in[i].Type != token.HASH {
			if enabled() {
				out = append(out, in[i])
			}
			i++
			continue
		}
		line, j := lineTokens(in, i)
		if len(line) < 2 {
			i = j
			continue
		}
		name := line[1].Literal
		args := line[2:]
		pos := in[i].Pos
		switch name {
		case "if", "ifdef", "ifndef":
			parent := enabled()
			ok := false
			if name == "ifdef" && len(args) == 1 {
				_, ok = e.Macros[args[0].Literal]
			}
			if name == "ifndef" && len(args) == 1 {
				_, ok = e.Macros[args[0].Literal]
				ok = !ok
			}
			if name == "if" {
				var err error
				ok, err = e.eval(args, &limit)
				if err != nil {
					return nil, err
				}
			}
			stack = append(stack, condition{parent: parent, active: parent && ok, taken: ok})
		case "elif":
			if len(stack) == 0 {
				return nil, fmt.Errorf("%s: #elif without #if", pos.String())
			}
			s := &stack[len(stack)-1]
			if s.taken {
				s.active = false
			} else {
				ok, err := e.eval(args, &limit)
				if err != nil {
					return nil, err
				}
				s.active = s.parent && ok
				s.taken = ok
			}
		case "else":
			if len(stack) == 0 {
				return nil, fmt.Errorf("%s: #else without #if", pos.String())
			}
			s := &stack[len(stack)-1]
			s.active = s.parent && !s.taken
			s.taken = true
		case "endif":
			if len(stack) == 0 {
				return nil, fmt.Errorf("%s: #endif without #if", pos.String())
			}
			stack = stack[:len(stack)-1]
		case "define":
			if enabled() {
				if err := e.parseDefine(args, pos); err != nil {
					return nil, err
				}
			}
		case "undef":
			if enabled() {
				if len(args) != 1 {
					return nil, fmt.Errorf("%s: invalid #undef", pos.String())
				}
				e.Undef(args[0].Literal)
			}
		case "include":
			if enabled() {
				if e.Include == nil {
					return nil, fmt.Errorf("%s: #include has no resolver", pos.String())
				}
				if len(args) != 1 || args[0].Type != token.STRING {
					return nil, fmt.Errorf("%s: invalid #include", pos.String())
				}
				more, err := e.Include(args[0].Literal, pos)
				if err != nil {
					return nil, err
				}
				expanded, err := e.directives(withoutEOF(more), limit)
				if err != nil {
					return nil, err
				}
				out = append(out, expanded...)
			}
		default:
			if enabled() {
				return nil, fmt.Errorf("%s: unknown directive #%s", pos.String(), name)
			}
		}
		i = j
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unterminated conditional directive")
	}
	return out, nil
}

func lineTokens(in []token.Token, i int) ([]token.Token, int) {
	line := in[i].Pos.Line
	j := i
	for j < len(in) && in[j].Pos.Line == line {
		j++
	}
	return in[i:j], j
}
func (e *Expander) parseDefine(a []token.Token, pos token.Position) error {
	if len(a) == 0 || a[0].Type != token.IDENT {
		return fmt.Errorf("%s: invalid #define", pos.String())
	}
	m := Macro{Name: a[0].Literal}
	i := 1
	if i < len(a) && a[i].Type == token.LPAREN && a[i].Pos.Offset == a[0].Pos.Offset+len(a[0].Literal) {
		m.Parameters = []string{}
		i++
		for {
			if i >= len(a) {
				return fmt.Errorf("%s: unterminated macro parameters", pos.String())
			}
			if a[i].Type == token.RPAREN {
				i++
				break
			}
			if a[i].Type == token.ELLIPSIS {
				m.Variadic = true
				m.Parameters = append(m.Parameters, "__VA_ARGS__")
				i++
				if i >= len(a) || a[i].Type != token.RPAREN {
					return fmt.Errorf("%s: invalid variadic macro", pos.String())
				}
				i++
				break
			}
			if a[i].Type != token.IDENT {
				return fmt.Errorf("%s: invalid macro parameter", pos.String())
			}
			m.Parameters = append(m.Parameters, a[i].Literal)
			i++
			if i < len(a) && a[i].Type == token.COMMA {
				i++
				continue
			}
			if i < len(a) && a[i].Type == token.RPAREN {
				i++
				break
			}
			return fmt.Errorf("%s: invalid macro parameters", pos.String())
		}
	}
	m.Replacement = append(m.Replacement, a[i:]...)
	e.Define(m)
	return nil
}

func (e *Expander) expand(in []token.Token, disabled map[string]bool, budget *int) ([]token.Token, error) {
	var out []token.Token
	for i := 0; i < len(in); i++ {
		t := in[i]
		m, ok := e.Macros[t.Literal]
		if !ok || disabled[m.Name] {
			out = append(out, t)
			continue
		}
		*budget--
		if *budget < 0 {
			return nil, fmt.Errorf("macro expansion limit exceeded")
		}
		var args [][]token.Token
		end := i + 1
		if m.FunctionLike() {
			if end >= len(in) || in[end].Type != token.LPAREN {
				out = append(out, t)
				continue
			}
			var good bool
			args, end, good = arguments(in, end)
			if !good {
				return nil, fmt.Errorf("%s: unterminated macro invocation", t.Pos.String())
			}
			if !m.Variadic && len(args) != len(m.Parameters) {
				return nil, fmt.Errorf("%s: macro %s expects %d arguments, got %d", t.Pos.String(), m.Name, len(m.Parameters), len(args))
			}
			if m.Variadic && len(args) < len(m.Parameters)-1 {
				return nil, fmt.Errorf("%s: macro %s has too few arguments", t.Pos.String(), m.Name)
			}
		}
		disabled[m.Name] = true
		repl, err := e.substitute(m, args, disabled, budget, t.Pos)
		delete(disabled, m.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, repl...)
		i = end - 1
	}
	return out, nil
}
func arguments(in []token.Token, open int) ([][]token.Token, int, bool) {
	var a [][]token.Token
	start := open + 1
	depth := 0
	for i := start; i < len(in); i++ {
		switch in[i].Type {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			if depth == 0 {
				if i > start || len(a) > 0 {
					a = append(a, in[start:i])
				}
				return a, i + 1, true
			}
			depth--
		case token.COMMA:
			if depth == 0 {
				a = append(a, in[start:i])
				start = i + 1
			}
		}
	}
	return nil, 0, false
}
func (e *Expander) substitute(m Macro, args [][]token.Token, disabled map[string]bool, budget *int, pos token.Position) ([]token.Token, error) {
	raw := map[string][]token.Token{}
	for i, p := range m.Parameters {
		if m.Variadic && p == "__VA_ARGS__" {
			var v []token.Token
			for j := i; j < len(args); j++ {
				if j > i {
					v = append(v, token.Token{Type: token.COMMA, Literal: ",", Pos: pos})
				}
				v = append(v, args[j]...)
			}
			raw[p] = v
		} else {
			raw[p] = args[i]
		}
	}
	exp := map[string][]token.Token{}
	for p, v := range raw {
		x, err := e.expand(v, disabled, budget)
		if err != nil {
			return nil, err
		}
		exp[p] = x
	}
	var r []token.Token
	for i := 0; i < len(m.Replacement); i++ {
		t := m.Replacement[i]
		if t.Type == token.HASH && i+1 < len(m.Replacement) {
			if v, ok := raw[m.Replacement[i+1].Literal]; ok {
				r = append(r, token.Token{Type: token.STRING, Literal: stringify(v), Pos: pos})
				i++
				continue
			}
		}
		if t.Type == token.HASH_HASH {
			if len(r) == 0 || i+1 >= len(m.Replacement) {
				return nil, fmt.Errorf("%s: invalid ## in macro %s", pos.String(), m.Name)
			}
			next := m.Replacement[i+1]
			v := []token.Token{next}
			if a, ok := raw[next.Literal]; ok {
				v = a
			}
			if len(v) == 0 {
				continue
			}
			pasted, err := paste(r[len(r)-1], v[0], pos)
			if err != nil {
				return nil, err
			}
			r[len(r)-1] = pasted
			r = append(r, v[1:]...)
			i++
			continue
		}
		if v, ok := exp[t.Literal]; ok {
			r = append(r, v...)
		} else {
			t.Pos = pos
			r = append(r, t)
		}
	}
	return e.expand(r, disabled, budget)
}
func stringify(ts []token.Token) string {
	var b strings.Builder
	for i, t := range ts {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t.Literal)
	}
	return b.String()
}
func paste(a, b token.Token, pos token.Position) (token.Token, error) {
	ts := lexer.New(pos.Filename, a.Literal+b.Literal).Tokens()
	if len(ts) != 2 || ts[0].Type == token.ILLEGAL {
		return token.Token{}, fmt.Errorf("%s: token paste %q is invalid", pos.String(), a.Literal+b.Literal)
	}
	ts[0].Pos = pos
	return ts[0], nil
}
func (e *Expander) eval(ts []token.Token, budget *int) (bool, error) {
	// The operand of defined is deliberately not macro-expanded (C99 6.10.1).
	// Replace it before the ordinary expansion pass so a macro named by the
	// operand cannot turn `defined(NAME)` into `defined(replacement)`.
	var protected []token.Token
	for i := 0; i < len(ts); i++ {
		if ts[i].Type == token.IDENT && ts[i].Literal == "defined" {
			j := i + 1
			paren := j < len(ts) && ts[j].Type == token.LPAREN
			if paren {
				j++
			}
			if j < len(ts) && ts[j].Type == token.IDENT {
				_, exists := e.Macros[ts[j].Literal]
				j++
				if paren {
					if j >= len(ts) || ts[j].Type != token.RPAREN {
						return false, fmt.Errorf("invalid defined expression")
					}
					j++
				}
				protected = append(protected, token.Token{Type: token.INT, Literal: strconv.FormatInt(boolInt(exists), 10), Pos: ts[i].Pos})
				i = j - 1
				continue
			}
			return false, fmt.Errorf("invalid defined expression")
		}
		protected = append(protected, ts[i])
	}
	x, err := e.expand(protected, map[string]bool{}, budget)
	if err != nil {
		return false, err
	}
	p := &expr{tokens: x, macros: e.Macros}
	v, err := p.or()
	if err != nil {
		return false, err
	}
	if p.i < len(x) {
		return false, fmt.Errorf("%s: invalid #if expression", x[p.i].Pos.String())
	}
	return v != 0, nil
}

type expr struct {
	tokens []token.Token
	i      int
	macros map[string]Macro
}

func (p *expr) or() (int64, error) {
	v, e := p.and()
	for e == nil && p.take(token.LOGICAL_OR) {
		r, x := p.and()
		v = boolInt(v != 0 || r != 0)
		e = x
	}
	return v, e
}
func (p *expr) and() (int64, error) {
	v, e := p.unary()
	for e == nil && p.take(token.LOGICAL_AND) {
		r, x := p.unary()
		v = boolInt(v != 0 && r != 0)
		e = x
	}
	return v, e
}
func (p *expr) unary() (int64, error) {
	if p.take(token.LOGICAL_NOT) {
		v, e := p.unary()
		return boolInt(v == 0), e
	}
	if p.take(token.LPAREN) {
		v, e := p.or()
		if !p.take(token.RPAREN) {
			return 0, fmt.Errorf("missing ) in #if expression")
		}
		return v, e
	}
	if p.i >= len(p.tokens) {
		return 0, fmt.Errorf("missing #if expression")
	}
	t := p.tokens[p.i]
	p.i++
	if t.Type == token.IDENT && t.Literal == "defined" {
		if p.take(token.LPAREN) {
			if p.i >= len(p.tokens) {
				return 0, fmt.Errorf("invalid defined expression")
			}
			n := p.tokens[p.i].Literal
			p.i++
			if !p.take(token.RPAREN) {
				return 0, fmt.Errorf("invalid defined expression")
			}
			_, ok := p.macros[n]
			return boolInt(ok), nil
		}
		if p.i >= len(p.tokens) {
			return 0, fmt.Errorf("invalid defined expression")
		}
		n := p.tokens[p.i].Literal
		p.i++
		_, ok := p.macros[n]
		return boolInt(ok), nil
	}
	if t.Type == token.INT {
		v, e := strconv.ParseInt(t.Literal, 0, 64)
		return v, e
	}
	return 0, nil
}
func (p *expr) take(k token.TokenType) bool {
	if p.i < len(p.tokens) && p.tokens[p.i].Type == k {
		p.i++
		return true
	}
	return false
}
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
func withoutEOF(in []token.Token) []token.Token {
	for len(in) > 0 && in[len(in)-1].Type == token.EOF {
		in = in[:len(in)-1]
	}
	return in
}
