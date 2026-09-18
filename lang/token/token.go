package token

// TokenType represents the type of a token
type Type string

const (
	// Keyword types
	TypeVoid    Type = "VOID"
	TypeInt     Type = "INT"
	TypeStruct  Type = "STRUCT"
	TypeTypedef Type = "TYPEDEF"
	TypeIf      Type = "IF"
	TypeElse    Type = "ELSE"
	TypeFor     Type = "FOR"
	TypeWhile   Type = "WHILE"
	TypeReturn  Type = "RETURN"
	TypeSwitch  Type = "SWITCH"
	TypeCase    Type = "CASE"
	TypeDefault Type = "DEFAULT"

	// Operator types
	TypePlus         Type = "PLUS"
	TypeMinus        Type = "MINUS"
	TypeStar         Type = "STAR"
	TypeSlash        Type = "SLASH"
	TypePercent      Type = "PERCENT"
	TypeEqual        Type = "EQUAL"
	TypeNotEqual     Type = "NOT_EQUAL"
	TypeLess         Type = "LESS"
	TypeGreater      Type = "GREATER"
	TypeLessEqual    Type = "LESS_EQUAL"
	TypeGreaterEqual Type = "GREATER_EQUAL"
	TypeAssign       Type = "ASSIGN"
	TypePlusAssign   Type = "PLUS_ASSIGN"
	TypeMinusAssign  Type = "MINUS_ASSIGN"
	TypeStarAssign   Type = "STAR_ASSIGN"
	TypeSlashAssign  Type = "SLASH_ASSIGN"
	TypeModAssign    Type = "MOD_ASSIGN"
	TypeIncDec       Type = "INC_DEC"

	// Punctuation types
	TypeLParen    Type = "LPAREN"
	TypeRParen    Type = "RPAREN"
	TypeLBrace    Type = "LBRACE"
	TypeRBrace    Type = "RBRACE"
	TypeLBracket  Type = "LBRACKET"
	TypeRBracket  Type = "RBRACKET"
	TypeComma     Type = "COMMA"
	TypeSemicolon Type = "SEMICOLON"
	TypeColon     Type = "COLON"

	// Literal types
	TypeIdentifier Type = "IDENTIFIER"
	TypeInteger    Type = "INTEGER"
	TypeFloat      Type = "FLOAT"
	TypeString     Type = "STRING"
	TypeCharacter  Type = "CHARACTER"

	// Preprocessor types
	TypeHash      Type = "HASH"
	TypeInclude   Type = "INCLUDE"
	TypeIfdef     Type = "IFDEF"
	TypeIfndef    Type = "IFNDEF"
	TypeDefine    Type = "DEFINE"
	TypeIfToken   Type = "PREPROCESS_IF"
	TypeElseToken Type = "PREPROCESS_ELSE"
	TypeEndif     Type = "PREPROCESS_ENDIF"

	// Special types
	TypeEof   Type = "EOF"
	TypeError Type = "ERROR"
)

// Token represents a lexical token with its type, value, and position
type Token struct {
	Type   Type
	Value  string
	Line   int
	Column int
}

// Keywords maps lowercase strings to their TokenType
var Keywords = map[string]Type{
	"void":    TypeVoid,
	"int":     TypeInt,
	"struct":  TypeStruct,
	"typedef": TypeTypedef,
	"if":      TypeIf,
	"else":    TypeElse,
	"for":     TypeFor,
	"while":   TypeWhile,
	"return":  TypeReturn,
	"switch":  TypeSwitch,
	"case":    TypeCase,
	"default": TypeDefault,
}

// Operators maps operator characters/strings to their TokenType
var Operators = map[string]Type{
	"+":  TypePlus,
	"-":  TypeMinus,
	"*":  TypeStar,
	"/":  TypeSlash,
	"%":  TypePercent,
	"==": TypeNotEqual,
	"!=": TypeNotEqual,
	"<":  TypeLess,
	">":  TypeGreater,
	"<=": TypeLessEqual,
	">=": TypeGreaterEqual,
	"=":  TypeAssign,
	"+=": TypePlusAssign,
	"-=": TypeMinusAssign,
	"*=": TypeStarAssign,
	"/=": TypeSlashAssign,
	"%=": TypeModAssign,
	"++": TypeIncDec,
	"--": TypeIncDec,
	"(":  TypeLParen,
	")":  TypeRParen,
	"{":  TypeLBrace,
	"}":  TypeRBrace,
	"[":  TypeLBracket,
	"]":  TypeRBracket,
	",":  TypeComma,
	";":  TypeSemicolon,
	":":  TypeColon,
}

// IsKeyword checks if the given string is a keyword
func IsKeyword(s string) bool {
	_, ok := Keywords[s]
	return ok
}

// IsOperator checks if the given string is an operator
func IsOperator(s string) bool {
	_, ok := Operators[s]
	return ok
}

// String returns a human-readable representation of the token type
func (t Type) String() string {
	return string(t)
}

// TypeFromString converts a string to a TokenType
func TypeFromString(s string) (Type, bool) {
	switch s {
	case "VOID", "void":
		return TypeVoid, true
	case "INT", "int":
		return TypeInt, true
	case "STRUCT", "struct":
		return TypeStruct, true
	case "TYPEDEF", "typedef":
		return TypeTypedef, true
	case "IF", "if":
		return TypeIf, true
	case "ELSE", "else":
		return TypeElse, true
	case "FOR", "for":
		return TypeFor, true
	case "WHILE", "while":
		return TypeWhile, true
	case "RETURN", "return":
		return TypeReturn, true
	case "SWITCH", "switch":
		return TypeSwitch, true
	case "CASE", "case":
		return TypeCase, true
	case "DEFAULT", "default":
		return TypeDefault, true
	default:
		return "", false
	}
}

// KeywordNames returns all keyword token types
func KeywordNames() []Type {
	names := make([]Type, 0, len(Keywords))
	for _, t := range Keywords {
		names = append(names, t)
	}
	return names
}

// OperatorNames returns all operator token types
func OperatorNames() []Type {
	names := make([]Type, 0, len(Operators))
	for _, t := range Operators {
		names = append(names, t)
	}
	return names
}

// Validate checks if a token type is valid
func (t Type) Validate() bool {
	switch t {
	case TypeVoid, TypeInt, TypeStruct, TypeTypedef,
		TypeIf, TypeElse, TypeFor, TypeWhile,
		TypeReturn, TypeSwitch, TypeCase, TypeDefault,
		TypePlus, TypeMinus, TypeStar, TypeSlash,
		TypePercent, TypeEqual, TypeNotEqual, TypeLess,
		TypeGreater, TypeLessEqual, TypeGreaterEqual,
		TypeAssign, TypePlusAssign, TypeMinusAssign,
		TypeStarAssign, TypeSlashAssign, TypeModAssign,
		TypeIncDec, TypeLParen, TypeRParen,
		TypeLBrace, TypeRBrace, TypeLBracket, TypeRBracket,
		TypeComma, TypeSemicolon, TypeColon,
		TypeIdentifier, TypeInteger, TypeFloat,
		TypeString, TypeCharacter,
		TypeHash, TypeInclude, TypeIfdef, TypeIfndef,
		TypeDefine, TypeIfToken, TypeElseToken, TypeEndif,
		TypeEof, TypeError:
		return true
	default:
		return false
	}
}
