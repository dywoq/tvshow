package semantic

import (
	"strings"
	"testing"

	"tvshow/lang/lexer"
	"tvshow/lang/parser"
	"tvshow/lang/token"
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

func TestAnalyzeFunctionAliasesAndStrictTypes(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:   "valid function pointer typedef and assignment",
			source: `typedef int (*BinaryOp)(int, int); int add(int a, int b) { return a + b; } int Start() { BinaryOp fn = add; return fn(1, 2); }`,
		},
		{
			name:   "valid address of function",
			source: `typedef int (*BinaryOp)(int, int); int add(int a, int b) { return a + b; } int Start() { BinaryOp fn = &add; return fn(1, 2); }`,
		},
		{
			name:   "dereferenced function pointer call",
			source: `typedef int (*BinaryOp)(int, int); int add(int a, int b) { return a + b; } int Start() { BinaryOp fn = add; return (*fn)(1, 2); }`,
		},
		{
			name:    "incompatible parameter count in function pointer assignment",
			source:  `typedef int (*Op2)(int, int); int single(int a) { return a; } int Start() { Op2 fn = single; return 0; }`,
			wantErr: `incompatible type in initialization of "fn"`,
		},
		{
			name:    "incompatible parameter type in function pointer assignment",
			source:  `typedef int (*Op)(int, string); int add(int a, int b) { return a + b; } int Start() { Op fn = add; return 0; }`,
			wantErr: `incompatible type in initialization of "fn"`,
		},
		{
			name:    "incompatible argument type in function pointer call",
			source:  `typedef int (*Op)(int, int); int add(int a, int b) { return a + b; } int Start() { Op fn = add; return fn(1, "hello"); }`,
			wantErr: `incompatible type for argument 2 in function call`,
		},
		{
			name:    "calling integer variable",
			source:  `int Start() { int x = 10; return x(5); }`,
			wantErr: `called object of type "int" is not a function`,
		},
		{
			name:    "assigning function to non-function variable",
			source:  `int add(int a, int b) { return a + b; } int Start() { int x = add; return x; }`,
			wantErr: `incompatible type in initialization of "x"`,
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

func TestAnalyzeStringify(t *testing.T) {
	source := `typedef struct Point { int x; int y; } Point;
void Start() {
	int a = 42;
	Point p;
	int arr[3];
	string s1 = stringify(a);
	string s2 = stringify(p);
	string s3 = stringify(arr);
}`
	if err := analyzeSource(t, source); err != nil {
		t.Fatalf("unexpected error analyzing stringify: %v", err)
	}
}

func TestAnalyzeDefer(t *testing.T) {
	source := `void calculate() { int result = 2 + 2; }
void start() {
	defer calculate();
}`
	if err := analyzeSource(t, source); err != nil {
		t.Fatalf("unexpected error analyzing valid defer statement: %v", err)
	}

	invalidCallSource := `void start() {
	defer missing_fn();
}`
	err := analyzeSource(t, invalidCallSource)
	if err == nil || !strings.Contains(err.Error(), `undefined identifier "missing_fn"`) {
		t.Fatalf("expected error for undefined deferred function call, got %v", err)
	}
}

func TestAnalyzeExceptions(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:   "valid string exception throw and catch",
			source: `int Divide(int a, int b) { if (b == 0) throw "division by zero"; return a / b; } void Start() { try { int res = Divide(10, 0); } catch (string exception) { string msg = exception; } }`,
		},
		{
			name:    "throw integer exception",
			source:  `void Start() { throw 42; }`,
			wantErr: `cannot throw non-string exception of type "int"`,
		},
		{
			name:    "throw struct exception",
			source:  `typedef struct Err { int code; } Err; void Start() { Err e; throw e; }`,
			wantErr: `cannot throw non-string exception of type "struct Err"`,
		},
		{
			name:    "catch non-string type",
			source:  `void Start() { try { int x = 1; } catch (int err) { int y = err; } }`,
			wantErr: `catch parameter type must be string (got "int")`,
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

func TestAnalyzeBuiltinPositionFunctions(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:   "valid built-in position functions usage",
			source: `int Start() { int l = line(); int c = column(); string f = filename(); return l + c; }`,
		},
		{
			name:    "redefinition of line function",
			source:  `int line() { return 0; } void Start() {}`,
			wantErr: `redefinition of "line"`,
		},
		{
			name:    "redefinition of line variable",
			source:  `int line = 10; void Start() {}`,
			wantErr: `redefinition of "line"`,
		},
		{
			name:    "redefinition of column function",
			source:  `int column() { return 0; } void Start() {}`,
			wantErr: `redefinition of "column"`,
		},
		{
			name:    "redefinition of filename function",
			source:  `string filename() { return ""; } void Start() {}`,
			wantErr: `redefinition of "filename"`,
		},
		{
			name:    "line called with arguments",
			source:  `int Start() { return line(1); }`,
			wantErr: `wrong number of arguments to function call: expected 0, got 1`,
		},
		{
			name:    "incompatible assignment from filename",
			source:  `int Start() { int x = filename(); return x; }`,
			wantErr: `incompatible type in initialization of "x"`,
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

func TestAnalyzeLambdaAndInternal(t *testing.T) {
	// Valid lambda usage
	validLambda := `
	typedef struct UserStruct { int id; } UserStruct;
	typedef int (*FuncAlias)(int);
	void start() {
		int result = 2 * 2;
		auto lambda = int(int given_result) {
			return given_result * 2 + result;
		};
		int r = lambda(result);

		FuncAlias alias = int(int x) { return x + 1; };
	}`
	if err := analyzeSource(t, validLambda); err != nil {
		t.Fatalf("unexpected error analyzing valid lambda: %v", err)
	}

	// Lambda return type mismatch error
	mismatchLambda := `
	typedef struct UserStruct { int id; } UserStruct;
	void start() {
		int result = 4;
		auto lambda = int(int given_result) {
			return given_result * 2;
		};
		UserStruct user = lambda(result);
	}`
	err := analyzeSource(t, mismatchLambda)
	t.Logf("mismatchLambda err = %v", err)
	if err == nil || !strings.Contains(err.Error(), `incompatible type in initialization of "user"`) {
		t.Fatalf("expected incompatible type error, got: %v", err)
	}

	// Lambda at global scope error
	globalLambda := `auto global_lambda = int(int x) { return x; };`
	err = analyzeSource(t, globalLambda)
	if err == nil || !strings.Contains(err.Error(), "lambda expression is only allowed within a function") {
		t.Fatalf("expected global lambda error, got: %v", err)
	}

	// Internal keyword cross-file access error
	defTokens := lexer.New("def.sc", "internal float PI = 3.14;\n").Tokens()
	mainTokens := lexer.New("main.sc", "float PI2 = PI;\n").Tokens()
	// Combine tokens without EOF in def
	var combined []token.Token
	for _, tok := range defTokens {
		if tok.Type != token.EOF {
			combined = append(combined, tok)
		}
	}
	combined = append(combined, mainTokens...)
	prog, err := parser.Parse(combined)
	if err != nil {
		t.Fatalf("parse combined error: %v", err)
	}
	err = Analyze(prog)
	if err == nil || !strings.Contains(err.Error(), `cannot access internal symbol "PI" from file "main.sc"`) {
		t.Fatalf("expected internal symbol access error, got: %v", err)
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
