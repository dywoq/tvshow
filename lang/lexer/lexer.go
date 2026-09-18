package lexer

import (
	"tvshow/lang/token"
	"unicode"
)

// Lexer represents a lexical analyzer for Scintilla (C99-inspired).
type Lexer struct {
	input        string
	filename     string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
	line         int  // current line number (1-based)
	column       int  // current column number (1-based)
}

// New creates a new Lexer instance for the given source input and filename.
func New(filename, input string) *Lexer {
	l := &Lexer{
		filename: filename,
		input:    input,
		line:     1,
		column:   0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0 // EOF
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
	if l.ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' || l.ch == '\v' || l.ch == '\f' {
			l.readChar()
		}

		// Check for comments
		if l.ch == '/' && l.peekChar() == '/' {
			// Line comment
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
		} else if l.ch == '/' && l.peekChar() == '*' {
			// Block comment
			l.readChar() // consume '/'
			l.readChar() // consume '*'
			for {
				if l.ch == 0 {
					break
				}
				if l.ch == '*' && l.peekChar() == '/' {
					l.readChar() // consume '*'
					l.readChar() // consume '/'
					break
				}
				l.readChar()
			}
		} else {
			break
		}
	}
}

// NextToken scans and returns the next token from the input.
func (l *Lexer) NextToken() token.Token {
	l.skipWhitespaceAndComments()

	pos := token.Position{
		Filename: l.filename,
		Line:     l.line,
		Column:   l.column,
		Offset:   l.position,
	}

	if l.ch == 0 {
		return token.Token{Type: token.EOF, Literal: "", Pos: pos}
	}

	var tok token.Token
	tok.Pos = pos

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.EQ, Literal: string(ch) + string(l.ch), Pos: pos}
		} else {
			tok = token.Token{Type: token.ASSIGN, Literal: "=", Pos: pos}
		}
	case '+':
		if l.peekChar() == '+' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.INCREMENT, Literal: string(ch) + string(l.ch), Pos: pos}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.PLUS_ASSIGN, Literal: string(ch) + string(l.ch), Pos: pos}
		} else {
			tok = token.Token{Type: token.PLUS, Literal: "+", Pos: pos}
		}
	case '-':
		if l.peekChar() == '-' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.DECREMENT, Literal: string(ch) + string(l.ch), Pos: pos}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MINUS_ASSIGN, Literal: string(ch) + string(l.ch), Pos: pos}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.ARROW, Literal: string(ch) + string(l.ch), Pos: pos}
		} else {
			tok = token.Token{Type: token.MINUS, Literal: "-", Pos: pos}
		}
	case '*':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.ASTERISK_ASSIGN, Literal: string(ch) + string(l.ch), Pos: pos}
		} else {
			tok = token.Token{Type: token.ASTERISK, Literal: "*", Pos: pos}
		}
	case '/':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.SLASH_ASSIGN, Literal: string(ch) + string(l.ch), Pos: pos}
		} else {
			tok = token.Token{Type: token.SLASH, Literal: "/", Pos: pos}
		}
	case '%':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.PERCENT_ASSIGN, Literal: string(ch) + string(l.ch), Pos: pos}
		} else {
			tok = token.Token{Type: token.PERCENT, Literal: "%", Pos: pos}
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.NOT_EQ, Literal: string(ch) + string(l.ch), Pos: pos}
		} else {
			tok = token.Token{Type: token.LOGICAL_NOT, Literal: "!", Pos: pos}
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.LTE, Literal: string(ch) + string(l.ch), Pos: pos}
		} else if l.peekChar() == '<' {
			l.readChar() // consume first '<'
			if l.peekChar() == '=' {
				l.readChar() // consume '='
				tok = token.Token{Type: token.SHL_ASSIGN, Literal: "<<=", Pos: pos}
			} else {
				tok = token.Token{Type: token.SHL, Literal: "<<", Pos: pos}
			}
		} else {
			tok = token.Token{Type: token.LT, Literal: "<", Pos: pos}
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.GTE, Literal: string(ch) + string(l.ch), Pos: pos}
		} else if l.peekChar() == '>' {
			l.readChar() // consume first '>'
			if l.peekChar() == '=' {
				l.readChar() // consume '='
				tok = token.Token{Type: token.SHR_ASSIGN, Literal: ">>=", Pos: pos}
			} else {
				tok = token.Token{Type: token.SHR, Literal: ">>", Pos: pos}
			}
		} else {
			tok = token.Token{Type: token.GT, Literal: ">", Pos: pos}
		}
	case '&':
		if l.peekChar() == '&' {
			l.readChar()
			tok = token.Token{Type: token.LOGICAL_AND, Literal: "&&", Pos: pos}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.BIT_AND_ASSIGN, Literal: "&=", Pos: pos}
		} else {
			tok = token.Token{Type: token.BIT_AND, Literal: "&", Pos: pos}
		}
	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			tok = token.Token{Type: token.LOGICAL_OR, Literal: "||", Pos: pos}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.BIT_OR_ASSIGN, Literal: "|=", Pos: pos}
		} else {
			tok = token.Token{Type: token.BIT_OR, Literal: "|", Pos: pos}
		}
	case '^':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.BIT_XOR_ASSIGN, Literal: "^=", Pos: pos}
		} else {
			tok = token.Token{Type: token.BIT_XOR, Literal: "^", Pos: pos}
		}
	case '~':
		tok = token.Token{Type: token.BIT_NOT, Literal: "~", Pos: pos}
	case '?':
		tok = token.Token{Type: token.QUESTION, Literal: "?", Pos: pos}
	case ':':
		tok = token.Token{Type: token.COLON, Literal: ":", Pos: pos}
	case ';':
		tok = token.Token{Type: token.SEMICOLON, Literal: ";", Pos: pos}
	case ',':
		tok = token.Token{Type: token.COMMA, Literal: ",", Pos: pos}
	case '.':
		if l.peekChar() == '.' {
			l.readChar()
			if l.peekChar() == '.' {
				l.readChar()
				tok = token.Token{Type: token.ELLIPSIS, Literal: "...", Pos: pos}
			} else {
				tok = token.Token{Type: token.ILLEGAL, Literal: "..", Pos: pos}
			}
		} else if isDigit(l.peekChar()) {
			return l.readNumber()
		} else {
			tok = token.Token{Type: token.DOT, Literal: ".", Pos: pos}
		}
	case '#':
		if l.peekChar() == '#' {
			l.readChar()
			tok = token.Token{Type: token.HASH_HASH, Literal: "##", Pos: pos}
		} else {
			tok = token.Token{Type: token.HASH, Literal: "#", Pos: pos}
		}
	case '(':
		tok = token.Token{Type: token.LPAREN, Literal: "(", Pos: pos}
	case ')':
		tok = token.Token{Type: token.RPAREN, Literal: ")", Pos: pos}
	case '[':
		tok = token.Token{Type: token.LBRACK, Literal: "[", Pos: pos}
	case ']':
		tok = token.Token{Type: token.RBRACK, Literal: "]", Pos: pos}
	case '{':
		tok = token.Token{Type: token.LBRACE, Literal: "{", Pos: pos}
	case '}':
		tok = token.Token{Type: token.RBRACE, Literal: "}", Pos: pos}
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
		return tok
	case '\'':
		tok.Type = token.CHAR
		tok.Literal = l.readCharLiteral()
		return tok
	default:
		if isLetter(l.ch) {
			literal := l.readIdentifier()
			tokType := token.LookupIdent(literal)
			return token.Token{Type: tokType, Literal: literal, Pos: pos}
		} else if isDigit(l.ch) {
			return l.readNumber()
		} else {
			ch := l.ch
			l.readChar()
			return token.Token{Type: token.ILLEGAL, Literal: string(ch), Pos: pos}
		}
	}

	l.readChar()
	return tok
}

// Tokens tokenizes the entire input and returns a slice of all tokens up to EOF.
func (l *Lexer) Tokens() []token.Token {
	var tokens []token.Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == token.EOF {
			break
		}
	}
	return tokens
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return ('0' <= ch && ch <= '9') || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

func isIntegerSuffix(ch byte) bool {
	chLower := unicode.ToLower(rune(ch))
	return chLower == 'u' || chLower == 'l'
}

func isNumberSuffix(ch byte) bool {
	chLower := unicode.ToLower(rune(ch))
	return chLower == 'u' || chLower == 'l' || chLower == 'f'
}

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ch == '_'
}

func (l *Lexer) readIdentifier() string {
	startPos := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[startPos:l.position]
}

func (l *Lexer) readNumber() token.Token {
	pos := token.Position{
		Filename: l.filename,
		Line:     l.line,
		Column:   l.column,
		Offset:   l.position,
	}

	startPos := l.position
	isFloat := false

	if l.ch == '0' && (l.peekChar() == 'x' || l.peekChar() == 'X') {
		l.readChar() // '0'
		l.readChar() // 'x'/'X'
		for isHexDigit(l.ch) {
			l.readChar()
		}
		for isIntegerSuffix(l.ch) {
			l.readChar()
		}
		return token.Token{
			Type:    token.INT,
			Literal: l.input[startPos:l.position],
			Pos:     pos,
		}
	}

	for isDigit(l.ch) {
		l.readChar()
	}

	if l.ch == '.' {
		isFloat = true
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	if l.ch == 'e' || l.ch == 'E' {
		isFloat = true
		l.readChar()
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	for isNumberSuffix(l.ch) {
		l.readChar()
	}

	tokType := token.INT
	if isFloat {
		tokType = token.FLOAT
	}

	return token.Token{
		Type:    tokType,
		Literal: l.input[startPos:l.position],
		Pos:     pos,
	}
}

func (l *Lexer) readString() string {
	l.readChar() // consume opening quote '"'
	startPos := l.position
	for {
		if l.ch == '"' || l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			l.readChar()
		}
		l.readChar()
	}
	s := l.input[startPos:l.position]
	if l.ch == '"' {
		l.readChar() // consume closing quote '"'
	}
	return s
}

func (l *Lexer) readCharLiteral() string {
	l.readChar() // consume opening single quote '\''
	startPos := l.position
	for {
		if l.ch == '\'' || l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			l.readChar()
		}
		l.readChar()
	}
	s := l.input[startPos:l.position]
	if l.ch == '\'' {
		l.readChar() // consume closing single quote '\''
	}
	return s
}
