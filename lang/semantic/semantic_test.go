package semantic

import (
	"strings"
	"testing"

	"tvshow/lang/lexer"
	"tvshow/lang/parser"
)

func analyzeSource(t *testing.T, source string) error {
	t.Helper()
	program, err := parser.Parse(lexer.New("semantic.sc", source).Tokens())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Analyze(program)
}

func TestAnalyzeAcceptsCompoundLiterals(t *testing.T) {
	err := analyzeSource(t, `typedef struct Point { int x; int y; } Point; int Start() { Point p = (Point){.x = 10, .y = 20}; (Point){.x = 1}.x = 2; return (int[]){1, 2, 3}[0]; }`)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
}

func TestAnalyzeAcceptsStringAndSizeof(t *testing.T) {
	err := analyzeSource(t, `string concat(string a, string b) { string res = a; res += b; res += '!'; int sz = sizeof(res) + sizeof(string); return res; }`)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
}

func TestAnalyzeAcceptsScopedFunctionAndForwardCall(t *testing.T) {
	err := analyzeSource(t, `int Start() { int value = next(1); { int value = 2; value++; } return value; } int next(int value) { return value; }`)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
}

func TestAnalyzeReportsIndependentProblems(t *testing.T) {
	err := analyzeSource(t, `int Start() { break; continue; goto missing; value = 1; 1 = value; }`)
	if err == nil {
		t.Fatal("Analyze() error = nil, want errors")
	}
	for _, want := range []string{"break statement", "continue statement", `undefined label "missing"`, `undefined identifier "value"`, "not assignable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestAnalyzeRejectsDuplicateAndOutOfContextCase(t *testing.T) {
	err := analyzeSource(t, `typedef int Number; int Start() { Number value; Number value; case 1: return 0; }`)
	if err == nil {
		t.Fatal("Analyze() error = nil, want errors")
	}
	if !strings.Contains(err.Error(), `redefinition of "value"`) || !strings.Contains(err.Error(), "not within a switch") {
		t.Errorf("unexpected errors: %v", err)
	}
}
