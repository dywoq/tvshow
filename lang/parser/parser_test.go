package parser

import (
	"testing"

	"tvshow/lang/lexer"
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
