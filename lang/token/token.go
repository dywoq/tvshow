package token

import (
	"fmt"
	"sync"
)

// TokenType represents the lexical type of a token.
type TokenType int

// Position represents a location in a source file (filename, line, column, offset).
type Position struct {
	Filename string
	Line     int
	Column   int
	Offset   int
}

// String returns the string representation of a Position.
func (p Position) String() string {
	if p.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Token represents a lexical token with its type, literal value, and source position.
type Token struct {
	Type    TokenType
	Literal string
	Pos     Position
}

// String returns a readable string representation of the token.
func (t Token) String() string {
	typeStr := t.Type.String()
	if t.Literal == "" || t.Literal == typeStr {
		return typeStr
	}
	return fmt.Sprintf("%s(%s)", typeStr, t.Literal)
}

// IsKeyword reports whether the token is a keyword.
func (t Token) IsKeyword() bool {
	return IsKeyword(t.Type)
}

// IsLiteral reports whether the token is a literal.
func (t Token) IsLiteral() bool {
	return IsLiteral(t.Type)
}

// IsOperator reports whether the token is an operator or punctuator.
func (t Token) IsOperator() bool {
	return IsOperator(t.Type)
}

// Token type constants (C99 & Scintilla extensions)
const (
	// Special tokens
	ILLEGAL TokenType = iota
	EOF
	COMMENT

	// Identifiers and Literals
	IDENT  // Identifiers (e.g. variable names, function names)
	INT    // Integer literal (e.g. 42, 0xFF, 077)
	FLOAT  // Floating-point literal (e.g. 3.14, 1e-10)
	CHAR   // Character literal (e.g. 'a')
	STRING // String literal (e.g. "hello")

	// Operators and Punctuators
	PLUS      // +
	MINUS     // -
	ASTERISK  // *
	SLASH     // /
	PERCENT   // %
	INCREMENT // ++
	DECREMENT // --

	ASSIGN          // =
	PLUS_ASSIGN     // +=
	MINUS_ASSIGN    // -=
	ASTERISK_ASSIGN // *=
	SLASH_ASSIGN    // /=
	PERCENT_ASSIGN  // %=

	EQ     // ==
	NOT_EQ // !=
	LT     // <
	GT     // >
	LTE    // <=
	GTE    // >=

	LOGICAL_AND // &&
	LOGICAL_OR  // ||
	LOGICAL_NOT // !

	BIT_AND // &
	BIT_OR  // |
	BIT_XOR // ^
	BIT_NOT // ~
	SHL     // <<
	SHR     // >>

	BIT_AND_ASSIGN // &=
	BIT_OR_ASSIGN  // |=
	BIT_XOR_ASSIGN // ^=
	SHL_ASSIGN     // <<=
	SHR_ASSIGN     // >>=

	ARROW     // ->
	DOT       // .
	QUESTION  // ?
	COLON     // :
	SEMICOLON // ;
	COMMA     // ,

	LPAREN // (
	RPAREN // )
	LBRACK // [
	RBRACK // ]
	LBRACE // {
	RBRACE // }

	ELLIPSIS  // ...
	HASH      // #
	HASH_HASH // ##

	// Keywords boundary marker
	keywordBeg

	// C99 Keywords
	AUTO
	BREAK
	CASE
	CHAR_KW
	CONST
	CONTINUE
	DEFAULT
	DO
	DOUBLE
	ELSE
	ENUM
	FLOAT_KW
	FOR
	GOTO
	IF
	INT_KW
	LONG
	RETURN
	SHORT
	SIGNED
	SIZEOF
	STRUCT
	SWITCH
	TYPEDEF
	UNION
	UNSIGNED
	VOID
	WHILE
	BOOL      // _Bool
	COMPLEX   // _Complex
	IMAGINARY // _Imaginary
	STRING_KW // string

	keywordEnd
)

var (
	mu     sync.RWMutex
	tokens = map[TokenType]string{
		ILLEGAL: "ILLEGAL",
		EOF:     "EOF",
		COMMENT: "COMMENT",

		IDENT:  "IDENT",
		INT:    "INT",
		FLOAT:  "FLOAT",
		CHAR:   "CHAR",
		STRING: "STRING",

		PLUS:      "+",
		MINUS:     "-",
		ASTERISK:  "*",
		SLASH:     "/",
		PERCENT:   "%",
		INCREMENT: "++",
		DECREMENT: "--",

		ASSIGN:          "=",
		PLUS_ASSIGN:     "+=",
		MINUS_ASSIGN:    "-=",
		ASTERISK_ASSIGN: "*=",
		SLASH_ASSIGN:    "/=",
		PERCENT_ASSIGN:  "%=",

		EQ:     "==",
		NOT_EQ: "!=",
		LT:     "<",
		GT:     ">",
		LTE:    "<=",
		GTE:    ">=",

		LOGICAL_AND: "&&",
		LOGICAL_OR:  "||",
		LOGICAL_NOT: "!",

		BIT_AND: "&",
		BIT_OR:  "|",
		BIT_XOR: "^",
		BIT_NOT: "~",
		SHL:     "<<",
		SHR:     ">>",

		BIT_AND_ASSIGN: "&=",
		BIT_OR_ASSIGN:  "|=",
		BIT_XOR_ASSIGN: "^=",
		SHL_ASSIGN:     "<<=",
		SHR_ASSIGN:     ">>=",

		ARROW:     "->",
		DOT:       ".",
		QUESTION:  "?",
		COLON:     ":",
		SEMICOLON: ";",
		COMMA:     ",",

		LPAREN: "(",
		RPAREN: ")",
		LBRACK: "[",
		RBRACK: "]",
		LBRACE: "{",
		RBRACE: "}",

		ELLIPSIS:  "...",
		HASH:      "#",
		HASH_HASH: "##",

		AUTO:      "auto",
		BREAK:     "break",
		CASE:      "case",
		CHAR_KW:   "char",
		CONST:     "const",
		CONTINUE:  "continue",
		DEFAULT:   "default",
		DO:        "do",
		DOUBLE:    "double",
		ELSE:      "else",
		ENUM:      "enum",
		FLOAT_KW:  "float",
		FOR:       "for",
		GOTO:      "goto",
		IF:        "if",
		INT_KW:    "int",
		LONG:      "long",
		RETURN:    "return",
		SHORT:     "short",
		SIGNED:    "signed",
		SIZEOF:    "sizeof",
		STRUCT:    "struct",
		SWITCH:    "switch",
		TYPEDEF:   "typedef",
		UNION:     "union",
		UNSIGNED:  "unsigned",
		VOID:      "void",
		WHILE:     "while",
		BOOL:      "_Bool",
		COMPLEX:   "_Complex",
		IMAGINARY: "_Imaginary",
		STRING_KW: "string",
	}
	keywords = map[string]TokenType{
		"auto":       AUTO,
		"break":      BREAK,
		"case":       CASE,
		"char":       CHAR_KW,
		"const":      CONST,
		"continue":   CONTINUE,
		"default":    DEFAULT,
		"do":         DO,
		"double":     DOUBLE,
		"else":       ELSE,
		"enum":       ENUM,
		"float":      FLOAT_KW,
		"for":        FOR,
		"goto":       GOTO,
		"if":         IF,
		"int":        INT_KW,
		"long":       LONG,
		"return":     RETURN,
		"short":      SHORT,
		"signed":     SIGNED,
		"sizeof":     SIZEOF,
		"struct":     STRUCT,
		"switch":     SWITCH,
		"typedef":    TYPEDEF,
		"union":      UNION,
		"unsigned":   UNSIGNED,
		"void":       VOID,
		"while":      WHILE,
		"_Bool":      BOOL,
		"_Complex":   COMPLEX,
		"_Imaginary": IMAGINARY,
		"string":     STRING_KW,
	}
)

// String returns the string representation of the token type.
func (tok TokenType) String() string {
	mu.RLock()
	defer mu.RUnlock()
	if s, ok := tokens[tok]; ok {
		return s
	}
	return fmt.Sprintf("TOKEN(%d)", tok)
}

// LookupIdent checks if an identifier is a keyword. If so, it returns the keyword's TokenType;
// otherwise, it returns IDENT.
func LookupIdent(ident string) TokenType {
	mu.RLock()
	defer mu.RUnlock()
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// IsKeyword reports whether the token type is a C99 keyword or registered keyword.
func IsKeyword(tok TokenType) bool {
	if tok > keywordBeg && tok < keywordEnd {
		return true
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, k := range keywords {
		if k == tok {
			return true
		}
	}
	return false
}

// IsLiteral reports whether the token type is a literal (INT, FLOAT, CHAR, STRING).
func IsLiteral(tok TokenType) bool {
	return tok >= IDENT && tok <= STRING
}

// IsOperator reports whether the token type is an operator or punctuator.
func IsOperator(tok TokenType) bool {
	return tok > STRING && tok < keywordBeg
}

// RegisterKeyword registers a custom or extension keyword and its TokenType, enabling extensibility.
func RegisterKeyword(name string, tok TokenType, stringRepr string) {
	mu.Lock()
	defer mu.Unlock()
	keywords[name] = tok
	tokens[tok] = stringRepr
}
