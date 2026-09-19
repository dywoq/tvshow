package token

import (
	"testing"
)

func TestPositionString(t *testing.T) {
	tests := []struct {
		pos      Position
		expected string
	}{
		{Position{Line: 10, Column: 5}, "10:5"},
		{Position{Filename: "main.sc", Line: 3, Column: 12}, "main.sc:3:12"},
	}

	for _, tt := range tests {
		if got := tt.pos.String(); got != tt.expected {
			t.Errorf("Position.String() = %q, expected %q", got, tt.expected)
		}
	}
}

func TestTokenString(t *testing.T) {
	tests := []struct {
		token    Token
		expected string
	}{
		{Token{Type: EOF}, "EOF"},
		{Token{Type: SEMICOLON, Literal: ";"}, ";"},
		{Token{Type: INT, Literal: "42"}, "INT(42)"},
		{Token{Type: IDENT, Literal: "Result"}, "IDENT(Result)"},
		{Token{Type: INT_KW}, "int"},
	}

	for _, tt := range tests {
		if got := tt.token.String(); got != tt.expected {
			t.Errorf("Token.String() = %q, expected %q", got, tt.expected)
		}
	}
}

func TestLookupIdent(t *testing.T) {
	tests := []struct {
		ident    string
		expected TokenType
	}{
		{"string", STRING_KW},
		{"int", INT_KW},
		{"struct", STRUCT},
		{"void", VOID},
		{"return", RETURN},
		{"_Bool", BOOL},
		{"register", IDENT},
		{"volatile", IDENT},
		{"restrict", IDENT},
		{"static", IDENT},
		{"extern", IDENT},
		{"inline", IDENT},
		{"CalculationResult", IDENT},
		{"myVariable", IDENT},
	}

	for _, tt := range tests {
		if got := LookupIdent(tt.ident); got != tt.expected {
			t.Errorf("LookupIdent(%q) = %v, expected %v", tt.ident, got, tt.expected)
		}
	}
}

func TestTokenClassification(t *testing.T) {
	if !IsKeyword(INT_KW) || !IsKeyword(STRING_KW) {
		t.Errorf("Expected INT_KW and STRING_KW to be keywords")
	}
	if IsKeyword(IDENT) {
		t.Errorf("Expected IDENT not to be a keyword")
	}

	if !IsLiteral(INT) || !IsLiteral(STRING) {
		t.Errorf("Expected INT and STRING to be literals")
	}
	if IsLiteral(PLUS) {
		t.Errorf("Expected PLUS not to be a literal")
	}

	if !IsOperator(PLUS) || !IsOperator(ASSIGN) || !IsOperator(LPAREN) {
		t.Errorf("Expected PLUS, ASSIGN, LPAREN to be operators/punctuators")
	}
	if IsOperator(INT_KW) {
		t.Errorf("Expected INT_KW not to be an operator")
	}

	tok := Token{Type: INT_KW}
	if !tok.IsKeyword() || tok.IsLiteral() || tok.IsOperator() {
		t.Errorf("Token classification methods incorrect for INT_KW")
	}
}

func TestExtensibilityRegisterKeyword(t *testing.T) {
	const customKeyword TokenType = 9999
	RegisterKeyword("__scintilla_ext", customKeyword, "__scintilla_ext")

	if got := LookupIdent("__scintilla_ext"); got != customKeyword {
		t.Errorf("LookupIdent(__scintilla_ext) = %v, expected %v", got, customKeyword)
	}

	if got := customKeyword.String(); got != "__scintilla_ext" {
		t.Errorf("customKeyword.String() = %q, expected %q", got, "__scintilla_ext")
	}

	if !IsKeyword(customKeyword) {
		t.Errorf("Expected registered customKeyword to be recognized as keyword")
	}
}
