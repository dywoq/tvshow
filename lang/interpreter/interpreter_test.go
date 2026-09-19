package interpreter

import (
	"testing"

	"tvshow/lang/bytecode"
	"tvshow/lang/lexer"
	"tvshow/lang/parser"
)

func programFromSource(t *testing.T, source string) *bytecode.Program {
	t.Helper()
	ast, err := parser.Parse(lexer.New("interpreter.sc", source).Tokens())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	program, err := bytecode.Translate(ast)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	return program
}

func TestRunExecutesGlobalsCallsAndControlFlow(t *testing.T) {
	program := programFromSource(t, `
		int offset = 2;
		int twice(int n) { return n * 2; }
		int Start(int n) {
			int total = offset;
			for (int i = 0; i < n; i++) { if (i == 2) continue; total += twice(i); }
			return total;
		}`)
	got, err := New(program).Run("Start", int64(4))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(10) {
		t.Errorf("Run = %#v, want 10", got)
	}
}

func TestInterpretBinaryMatchesInstructions(t *testing.T) {
	code := []bytecode.Instruction{
		{Opcode: bytecode.PushLiteral, Operand: "20"},
		{Opcode: bytecode.PushLiteral, Operand: "22"},
		{Opcode: bytecode.Binary, Operand: "+"},
		{Opcode: bytecode.Return},
	}
	encoded := make([][]byte, len(code))
	for n, instruction := range code {
		var err error
		encoded[n], err = instruction.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary: %v", err)
		}
	}
	got, err := InterpretBinary(encoded)
	if err != nil {
		t.Fatalf("InterpretBinary: %v", err)
	}
	if got != int64(42) {
		t.Errorf("InterpretBinary = %#v, want 42", got)
	}
}

func TestExecuteAddressesPreservePostfixValue(t *testing.T) {
	program := programFromSource(t, `int Start() { int value = 4; int old = value++; return old * 10 + value; }`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(45) {
		t.Errorf("Run = %#v, want 45", got)
	}
}

func TestRunExecutesDocExampleStructAndMultiply(t *testing.T) {
	program := programFromSource(t, `
		typedef struct CalculationResult { int A; int B; } CalculationResult;
		int Multiply(int a, int b) { return a * b; }
		int Start() {
			CalculationResult result;
			result.A = 2;
			result.B = 3;
			return Multiply(result.A, result.B);
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(6) {
		t.Errorf("Run = %#v, want 6", got)
	}
}

func TestRunExecutesStructCompoundLiteralsAndDesignators(t *testing.T) {
	program := programFromSource(t, `
		typedef struct Point { int x; int y; } Point;
		int Start() {
			Point p1 = { 10, 20 };
			Point p2 = (Point){ .y = 30, .x = 5 };
			return p1.x + p1.y + p2.x + p2.y;
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(65) {
		t.Errorf("Run = %#v, want 65", got)
	}
}

func TestRunExecutesArrayCompoundLiteralsAndDesignators(t *testing.T) {
	program := programFromSource(t, `
		int Start() {
			int a[] = { 1, 2, 3 };
			int b[5] = { [1] = 10, [3] = 20 };
			int c = (int[]){ 100, 200, 300 }[1];
			return a[0] + a[1] + a[2] + b[1] + b[3] + c;
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(1 + 2 + 3 + 10 + 20 + 200) {
		t.Errorf("Run = %#v, want %d", got, 1+2+3+10+20+200)
	}
}

func TestRunStringOperationsAndSizeof(t *testing.T) {
	program := programFromSource(t, `
		string Start() {
			string s = "hello";
			s += " ";
			s = s + "world";
			s += '!';
			int len = sizeof(s);
			if (len != 12) return "fail_len";
			if (sizeof(string) != 8) return "fail_sizeof_type";
			if (sizeof("test") != 4) return "fail_sizeof_lit";
			if (s[0] != 'h') return "fail_index";
			if ("abc" >= "def") return "fail_cmp";
			return s;
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "hello world!" {
		t.Errorf("Run = %#v, want %q", got, "hello world!")
	}
}

func TestRunAddressOfCompoundLiteral(t *testing.T) {
	program := programFromSource(t, `
		typedef struct Point { int x; int y; } Point;
		int get_x(Point *p) { return p->x; }
		int Start() {
			return get_x(&(Point){ .x = 42, .y = 10 });
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}

func TestHostFunctionCallFromGuest(t *testing.T) {
	program := programFromSource(t, `
		int add_host(int a, int b);
		int Start() {
			return add_host(15, 27);
		}`)
	interp := New(program)
	interp.RegisterFunction("add_host", func(args []any) (any, error) {
		a, err := integer(args[0])
		if err != nil {
			return nil, err
		}
		b, err := integer(args[1])
		if err != nil {
			return nil, err
		}
		return a + b, nil
	})

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}

func TestHostCallsGuest(t *testing.T) {
	program := programFromSource(t, `
		int double_it(int n) { return n * 2; }
		int call_host_proxy(int value);
		int Start(int n) {
			return call_host_proxy(n);
		}`)
	interp := New(program)
	interp.RegisterFunction("call_host_proxy", func(args []any) (any, error) {
		n := args[0]
		return interp.Run("double_it", n)
	})

	got, err := interp.Run("Start", int64(21))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}

func TestRunHostFunctionWithoutProgram(t *testing.T) {
	interp := New()
	interp.RegisterFunction("multiply", func(args []any) (any, error) {
		a, err := integer(args[0])
		if err != nil {
			return nil, err
		}
		b, err := integer(args[1])
		if err != nil {
			return nil, err
		}
		return a * b, nil
	})

	got, err := interp.Run("multiply", 6, 7)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}

func TestCallFunctionValueInVariable(t *testing.T) {
	fn := Function(func(args []any) (any, error) {
		a, _ := integer(args[0])
		return a + 10, nil
	})

	program := programFromSource(t, `
		int get_fn();
		int Start() {
			int f = get_fn();
			return f(32);
		}`)
	interp := New(program)
	interp.RegisterFunction("get_fn", func(args []any) (any, error) {
		return fn, nil
	})

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}

func TestHostGuestTypeConversions(t *testing.T) {
	program := programFromSource(t, `
		int sum(int a, int b) { return a + b; }
	`)
	interp := New(program)

	// Call guest function with various Go integer types (int, int32, int64)
	got, err := interp.Run("sum", int(10), int32(20))
	if err != nil {
		t.Fatalf("Run with int/int32: %v", err)
	}
	if got != int64(30) {
		t.Errorf("sum = %#v, want 30", got)
	}
}

func TestRunExecutesStructWithArrayDeclarations(t *testing.T) {
	program := programFromSource(t, `
		extern string __get_name();

		typedef struct _VECTORSTR {
			string List[];
		} VECTORSTR;

		string Start() {
			VECTORSTR Vector = (VECTORSTR){0};
			Vector.List[0] = "Hi!";
			return Vector.List[0];
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "Hi!" {
		t.Errorf("Run = %#v, want %q", got, "Hi!")
	}
}

func TestRunExecutesStructWithUninitializedArrayMember(t *testing.T) {
	program := programFromSource(t, `
		typedef struct _VECTORSTR {
			string List[];
		} VECTORSTR;

		string Start() {
			VECTORSTR Vector;
			Vector.List[0] = "Hello";
			Vector.List[1] = "World";
			return Vector.List[0] + " " + Vector.List[1];
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "Hello World" {
		t.Errorf("Run = %#v, want %q", got, "Hello World")
	}
}

func TestRunExecutesAutoType(t *testing.T) {
	program := programFromSource(t, `
		auto identity(auto val) {
			return val;
		}

		auto Start() {
			auto a = 10;
			auto b = "hello";
			a = b;
			auto c = identity(123);
			auto d = identity("world");
			return a + " " + d;
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "hello world" {
		t.Errorf("Run = %#v, want %q", got, "hello world")
	}
}

func TestRunExecutesStructWithArrayMemberInitialization(t *testing.T) {
	program := programFromSource(t, `
		typedef struct VectorInt {
			int List[3];
		} VectorInt;

		int Start() {
			VectorInt v = { .List = {10, 20, 30} };
			v.List[1] = 42;
			return v.List[0] + v.List[1] + v.List[2];
		}`)
	got, err := New(program).Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(10+42+30) {
		t.Errorf("Run = %#v, want %d", got, 10+42+30)
	}
}
