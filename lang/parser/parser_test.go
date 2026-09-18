package parser

import (
	"testing"

	"tvshow/lang/lexer"
	"tvshow/lang/token"
)

func TestParseTranslationUnit(t *testing.T) {
	input := `typedef struct Result { int a; int b; } Result;
int sum(Result *value, int n) {
  int total = value->a + value->b;
  for (int i = 0; i < n; i++) total += i;
  return total ? total : 0;
}`
	program, err := Parse(lexer.New("sample.sc", input).Tokens())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(program.Declarations) != 2 {
		t.Fatalf("declarations = %d, want 2", len(program.Declarations))
	}
	fn, ok := program.Declarations[1].(*FunctionDecl)
	if !ok || len(fn.Body.Items) != 3 {
		t.Fatalf("function body = %#v, want three items", fn)
	}
}

func TestNodesImplementStringer(t *testing.T) {
	var _ Node = &Program{}
	var _ Node = &VarDecl{Specs: []TypeSpec{{Token: token.Token{}}}}
	var _ Node = &FunctionDecl{Specs: []TypeSpec{{Token: token.Token{}}}}
	var _ Node = &BlockStmt{}
	var _ Node = &ExprStmt{}
	var _ Node = &IdentExpr{}
	var _ Node = &ArraySuffix{}
	var _ Node = &FunctionSuffix{}

	program, err := Parse(lexer.New("string.sc", "int answer(void) { return 42; }").Tokens())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := program.String(), "int answer {return 42;}"; got != want {
		t.Errorf("Program.String() = %q, want %q", got, want)
	}
}
