package macro

import (
	"reflect"
	"testing"

	"tvshow/lang/lexer"
	"tvshow/lang/token"
)

func source(s string) []token.Token { return lexer.New("test.sc", s).Tokens() }
func literals(ts []token.Token) []string {
	r := make([]string, len(ts))
	for i, t := range ts {
		r[i] = t.Literal
	}
	return r
}

func TestExpandDirectivesAndFunctionMacros(t *testing.T) {
	e := New()
	got, err := e.Expand(source(`#define VALUE 42
#define ADD(a, b) ((a) + (b))
#if defined(VALUE) && 1
ADD(VALUE, 3)
#else
wrong
#endif
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"(", "(", "42", ")", "+", "(", "3", ")", ")", ""}
	if !reflect.DeepEqual(literals(got), want) {
		t.Fatalf("literals = %#v, want %#v", literals(got), want)
	}
}

func TestErrorDirective(t *testing.T) {
	e := New()
	_, err := e.Expand(source("#error Something went wrong"))
	if err == nil || err.Error() != "test.sc:1:1: #error Something went wrong" {
		t.Errorf("unexpected error for #error with message: %v", err)
	}

	_, err = e.Expand(source("#error"))
	if err == nil || err.Error() != "test.sc:1:1: #error" {
		t.Errorf("unexpected error for empty #error: %v", err)
	}

	// In disabled block, #error should be ignored
	_, err = e.Expand(source("#if 0\n#error should not fire\n#endif\nint x = 5;"))
	if err != nil {
		t.Fatalf("expected no error in disabled block, got: %v", err)
	}
}

func TestStringifyPasteVariadicAndInclude(t *testing.T) {
	e := New()
	e.Include = func(name string, _ token.Position) ([]token.Token, error) {
		if name != "defs.sc" {
			t.Fatalf("include name %q", name)
		}
		return source("#define CAT(a,b) a ## b\n"), nil
	}
	got, err := e.Expand(source("#include \"defs.sc\"\n#define STR(x) #x\n#define LOG(...) __VA_ARGS__\nCAT(hel, lo) STR(a + b) LOG(1, 2)"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"hello", "a + b", "1", ",", "2", ""}
	if !reflect.DeepEqual(literals(got), want) {
		t.Fatalf("literals = %#v, want %#v", literals(got), want)
	}
}

func TestConditionalAndRecursiveMacros(t *testing.T) {
	e := New()
	got, err := e.Expand(source("#define A B\n#define B A\n#ifndef MISSING\nA\n#endif\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A", ""}; !reflect.DeepEqual(literals(got), want) {
		t.Fatalf("literals = %#v, want %#v", literals(got), want)
	}
}
