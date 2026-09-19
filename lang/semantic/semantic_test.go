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

func TestAnalyzeConstQualifiers(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "assign to const variable",
			source:  `int Start() { const int x = 10; x = 20; return x; }`,
			wantErr: "cannot assign to read-only target",
		},
		{
			name:    "increment const variable",
			source:  `int Start() { const int x = 10; x++; return x; }`,
			wantErr: "cannot modify read-only value",
		},
		{
			name:    "assign to field of const struct",
			source:  `typedef struct S { int a; } S; int Start() { const S s = (S){10}; s.a = 20; return s.a; }`,
			wantErr: "cannot assign to read-only target",
		},
		{
			name:    "assign to const struct field",
			source:  `typedef struct S { const int a; } S; int Start() { S s = (S){10}; s.a = 20; return s.a; }`,
			wantErr: "cannot assign to read-only target",
		},
		{
			name:    "uninitialized const local",
			source:  `int Start() { const int x; return x; }`,
			wantErr: `uninitialized const variable "x"`,
		},
		{
			name:    "assign through pointer to const",
			source:  `int Start(const int *p) { *p = 10; return *p; }`,
			wantErr: "cannot assign to read-only target",
		},
		{
			name:   "assign to pointer to const allowed",
			source: `int Start(const int *p) { int y = 20; p = &y; return *p; }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzeSource(t, tt.source)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestAnalyzeArrayChecks(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "subscript non-array non-pointer",
			source:  `int Start() { int x = 5; return x[0]; }`,
			wantErr: "subscripted value is not an array or pointer",
		},
		{
			name:    "non-integer array index",
			source:  `int Start() { int a[5]; return a[3.14]; }`,
			wantErr: "array subscript is not an integer",
		},
		{
			name:    "negative array size",
			source:  `int Start() { int a[-5]; return 0; }`,
			wantErr: "size of array is negative",
		},
		{
			name:    "non-integer array size type",
			source:  `int Start() { int a[3.14]; return 0; }`,
			wantErr: "size of array has non-integer type",
		},
		{
			name:    "array element cannot be void",
			source:  `int Start() { void arr[5]; return 0; }`,
			wantErr: `array "arr" element cannot be void`,
		},
		{
			name:    "excess array initializers",
			source:  `int Start() { int a[2] = {1, 2, 3}; return a[0]; }`,
			wantErr: `excess elements in array initializer for "a"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzeSource(t, tt.source)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestAnalyzeStructFieldChecks(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "duplicate struct member",
			source:  `typedef struct S { int a; float a; } S; int Start() { return 0; }`,
			wantErr: `duplicate member "a"`,
		},
		{
			name:    "struct member void type",
			source:  `typedef struct S { void v; } S; int Start() { return 0; }`,
			wantErr: `member "v" has invalid void type`,
		},
		{
			name:    "dot operator on non-struct",
			source:  `int Start() { int x = 5; return x.a; }`,
			wantErr: "expected struct or union before '.' operator",
		},
		{
			name:    "arrow operator on non-pointer struct",
			source:  `typedef struct S { int a; } S; int Start() { S s; return s->a; }`,
			wantErr: `member reference type "struct S" is not a pointer; did you mean '.'?`,
		},
		{
			name:    "dot operator on struct pointer",
			source:  `typedef struct S { int a; } S; int Start(S *p) { return p.a; }`,
			wantErr: `member reference type "struct S *" is a pointer; did you mean '->'?`,
		},
		{
			name:    "non-existent struct member",
			source:  `typedef struct S { int a; } S; int Start() { S s; return s.b; }`,
			wantErr: `"b" is not a member of struct S`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzeSource(t, tt.source)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestAnalyzeFunctionParametersAndCalls(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "void parameter with name",
			source:  `int fn(void x) { return 0; }`,
			wantErr: `parameter "x" declared with void type`,
		},
		{
			name:    "call non-function",
			source:  `int Start() { float x = 1.0; return x(); }`,
			wantErr: `called object of type "float" is not a function`,
		},
		{
			name:    "wrong argument count",
			source:  `int add(int a, int b) { return a + b; } int Start() { return add(1); }`,
			wantErr: "wrong number of arguments to function call: expected 2, got 1",
		},
		{
			name:    "incompatible argument type",
			source:  `typedef struct S { int x; } S; int add(int a) { return a; } int Start() { S s; return add(s); }`,
			wantErr: "incompatible type for argument 1 in function call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzeSource(t, tt.source)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}

	// Test valid single unnamed void parameter
	if err := analyzeSource(t, `int fn(void) { return 0; } int Start() { return fn(); }`); err != nil {
		t.Fatalf("unexpected error for fn(void): %v", err)
	}
}

func TestAnalyzeAutoType(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "auto variable holds int",
			source: `int Start() { auto x = 10; return x; }`,
		},
		{
			name:   "auto variable holds string and reassigned to int",
			source: `int Start() { auto x = "hello"; x = 42; return 0; }`,
		},
		{
			name:   "auto function param and return",
			source: `auto process(auto x) { return x; } int Start() { auto res = process(100); return 0; }`,
		},
		{
			name:   "auto bitwise operation",
			source: `int Start() { auto x = 5; auto y = x & 1; return y; }`,
		},
		{
			name:   "auto subscript and member access",
			source: `int Start() { auto arr; auto val = arr[0]; auto obj; auto m = obj.field; auto p = obj->field; return 0; }`,
		},
		{
			name:   "auto call operator",
			source: `int Start() { auto fn; auto res = fn(1, "test"); return 0; }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzeSource(t, tt.source)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.name, err)
			}
		})
	}
}

func TestAnalyzeLocalVariablesAndReturnStatements(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "variable declared void",
			source:  `int Start() { void x; return 0; }`,
			wantErr: `variable "x" declared with void type`,
		},
		{
			name:    "incompatible initialization",
			source:  `typedef struct S { int x; } S; int Start() { S s; int x = s; return x; }`,
			wantErr: `incompatible type in initialization of "x"`,
		},
		{
			name:    "void function returns value",
			source:  `void fn() { return 10; }`,
			wantErr: "void function should not return a value",
		},
		{
			name:    "non-void function returns no value",
			source:  `int fn() { return; }`,
			wantErr: "non-void function should return a value",
		},
		{
			name:    "incompatible return type",
			source:  `typedef struct S { int x; } S; int fn() { S s; return s; }`,
			wantErr: "incompatible return type in function returning int",
		},
		{
			name:    "bitwise operation on float",
			source:  `int Start() { float x = 3.14; return x % 2; }`,
			wantErr: "invalid operands to binary %",
		},
		{
			name:    "switch on non-integer",
			source:  `int Start() { float x = 3.14; switch (x) { case 1: return 1; } return 0; }`,
			wantErr: "switch quantity is not an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzeSource(t, tt.source)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
