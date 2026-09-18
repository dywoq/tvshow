package lexer

import (
	"testing"
	"tvshow/lang/token"
)

func TestLexerExampleProgram(t *testing.T) {
	input := `#include "def.sc"

void Start() {
	CalculationResult Result;
	Result.A = 2;
	Result.B = 2;
	int Result = Result.A + Result.B;
}
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.HASH, "#"},
		{token.IDENT, "include"},
		{token.STRING, "def.sc"},
		{token.VOID, "void"},
		{token.IDENT, "Start"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.IDENT, "CalculationResult"},
		{token.IDENT, "Result"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "Result"},
		{token.DOT, "."},
		{token.IDENT, "A"},
		{token.ASSIGN, "="},
		{token.INT, "2"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "Result"},
		{token.DOT, "."},
		{token.IDENT, "B"},
		{token.ASSIGN, "="},
		{token.INT, "2"},
		{token.SEMICOLON, ";"},
		{token.INT_KW, "int"},
		{token.IDENT, "Result"},
		{token.ASSIGN, "="},
		{token.IDENT, "Result"},
		{token.DOT, "."},
		{token.IDENT, "A"},
		{token.PLUS, "+"},
		{token.IDENT, "Result"},
		{token.DOT, "."},
		{token.IDENT, "B"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	l := New("main.sc", input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tok.Type wrong. expected=%q (%v), got=%q (%v)",
				i, tt.expectedType, tt.expectedType, tok.Type, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - tok.Literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexerOperatorsAndComments(t *testing.T) {
	input := `// line comment
	/* block comment */
	++ -- += -= *= /= %= <<= >>= && || == != <= >= -> ... ##`

	l := New("test.sc", input)
	toks := l.Tokens()

	expectedTypes := []token.TokenType{
		token.INCREMENT,
		token.DECREMENT,
		token.PLUS_ASSIGN,
		token.MINUS_ASSIGN,
		token.ASTERISK_ASSIGN,
		token.SLASH_ASSIGN,
		token.PERCENT_ASSIGN,
		token.SHL_ASSIGN,
		token.SHR_ASSIGN,
		token.LOGICAL_AND,
		token.LOGICAL_OR,
		token.EQ,
		token.NOT_EQ,
		token.LTE,
		token.GTE,
		token.ARROW,
		token.ELLIPSIS,
		token.HASH_HASH,
		token.EOF,
	}

	if len(toks) != len(expectedTypes) {
		t.Fatalf("expected %d tokens, got %d", len(expectedTypes), len(toks))
	}

	for i, expectedType := range expectedTypes {
		if toks[i].Type != expectedType {
			t.Errorf("token[%d] expected type %v, got %v (%s)", i, expectedType, toks[i].Type, toks[i].Literal)
		}
	}
}
