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

func TestParseCompoundLiteralsAndDesignators(t *testing.T) {
	input := `typedef struct Point { int x; int y; } Point;
void Start() {
	Point p = (Point){ .x = 10, .y = 20 };
	int arr[] = (int[]){ [0] = 1, [2] = 3 };
}`
	program, err := Parse(lexer.New("compound.sc", input).Tokens())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(program.Declarations) != 2 {
		t.Fatalf("declarations = %d, want 2", len(program.Declarations))
	}
	fn := program.Declarations[1].(*FunctionDecl)
	if len(fn.Body.Items) != 2 {
		t.Fatalf("body items = %d, want 2", len(fn.Body.Items))
	}
	varDecl1 := fn.Body.Items[0].(*VarDecl)
	comp1, ok := varDecl1.Declarators[0].Initializer.(*CompoundLiteralExpr)
	if !ok {
		t.Fatalf("expected CompoundLiteralExpr, got %T", varDecl1.Declarators[0].Initializer)
	}
	initList1 := comp1.Initializer.(*InitializerListExpr)
	if len(initList1.Values) != 2 || len(initList1.Values[0].Designators) != 1 || initList1.Values[0].Designators[0].Field.Literal != "x" {
		t.Errorf("unexpected initializer list 1: %#v", initList1)
	}

	varDecl2 := fn.Body.Items[1].(*VarDecl)
	comp2, ok := varDecl2.Declarators[0].Initializer.(*CompoundLiteralExpr)
	if !ok {
		t.Fatalf("expected CompoundLiteralExpr, got %T", varDecl2.Declarators[0].Initializer)
	}
	initList2 := comp2.Initializer.(*InitializerListExpr)
	if len(initList2.Values) != 2 || len(initList2.Values[0].Designators) != 1 || initList2.Values[0].Designators[0].Index == nil {
		t.Errorf("unexpected initializer list 2: %#v", initList2)
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
