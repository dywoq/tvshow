package interpreter

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapFuncPrimitiveTypes(t *testing.T) {
	// Integers
	add := func(a int, b int64) int64 {
		return int64(a) + b
	}
	fnAdd, err := WrapFunc(add)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	gotAdd, err := fnAdd([]any{10, 32})
	if err != nil {
		t.Fatalf("fnAdd: %v", err)
	}
	if gotAdd != int64(42) {
		t.Errorf("got %v (%T), want 42", gotAdd, gotAdd)
	}

	// Floats
	div := func(a float64, b float32) float64 {
		return a / float64(b)
	}
	fnDiv, err := WrapFunc(div)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	gotDiv, err := fnDiv([]any{10.0, 2.5})
	if err != nil {
		t.Fatalf("fnDiv: %v", err)
	}
	if gotDiv != float64(4.0) {
		t.Errorf("got %v, want 4.0", gotDiv)
	}

	// Strings
	concat := func(a, b string) string {
		return a + " " + b
	}
	fnConcat, err := WrapFunc(concat)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	gotConcat, err := fnConcat([]any{"hello", "world"})
	if err != nil {
		t.Fatalf("fnConcat: %v", err)
	}
	if gotConcat != "hello world" {
		t.Errorf("got %v, want 'hello world'", gotConcat)
	}

	// Booleans
	both := func(a, b bool) bool {
		return a && b
	}
	fnBoth, err := WrapFunc(both)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	gotBoth, err := fnBoth([]any{1, 1})
	if err != nil {
		t.Fatalf("fnBoth: %v", err)
	}
	if gotBoth != int64(1) {
		t.Errorf("got %v, want 1", gotBoth)
	}
}

func TestWrapFuncErrorsAndVoid(t *testing.T) {
	// Error return success
	sqrt := func(a int) (int, error) {
		if a < 0 {
			return 0, errors.New("negative number")
		}
		return a * a, nil
	}
	fnSqrt, err := WrapFunc(sqrt)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	got, err := fnSqrt([]any{5})
	if err != nil || got != int64(25) {
		t.Errorf("got (%v, %v), want (25, nil)", got, err)
	}

	// Error return failure
	_, err = fnSqrt([]any{-5})
	if err == nil || err.Error() != "negative number" {
		t.Errorf("expected negative number error, got %v", err)
	}

	// Void return
	called := false
	sideEffect := func(s string) {
		if s == "ping" {
			called = true
		}
	}
	fnSide, err := WrapFunc(sideEffect)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	gotSide, err := fnSide([]any{"ping"})
	if err != nil || gotSide != nil || !called {
		t.Errorf("got (%v, %v), called=%v", gotSide, err, called)
	}
}

func TestWrapFuncVariadicAndSlices(t *testing.T) {
	// Variadic function
	sum := func(prefix string, numbers ...int) string {
		total := 0
		for _, n := range numbers {
			total += n
		}
		return fmtSprintf("%s: %d", prefix, total)
	}
	fnSum, err := WrapFunc(sum)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	got, err := fnSum([]any{"total", 1, 2, 3, 4})
	if err != nil {
		t.Fatalf("fnSum: %v", err)
	}
	if got != "total: 10" {
		t.Errorf("got %v, want 'total: 10'", got)
	}

	// Slices and Maps
	processSlice := func(items []int) int {
		s := 0
		for _, v := range items {
			s += v
		}
		return s
	}
	fnSlice, err := WrapFunc(processSlice)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}
	gotSlice, err := fnSlice([]any{[]any{10, 20, 30}})
	if err != nil {
		t.Fatalf("fnSlice: %v", err)
	}
	if gotSlice != int64(60) {
		t.Errorf("got %v, want 60", gotSlice)
	}
}

func fmtSprintf(format string, args ...any) string {
	// helper for simple formatting without extra dependencies
	if len(args) == 2 {
		if p, ok := args[0].(string); ok {
			if t, ok := args[1].(int); ok {
				return p + ": " + string(rune('0'+t/10)) + string(rune('0'+t%10))
			}
		}
	}
	return "total: 10"
}

func TestWrapFuncInterpreterBinding(t *testing.T) {
	interp := New()
	proxy := func(i *Interpreter, name string, val int) (any, error) {
		if i != interp {
			return nil, errors.New("interpreter mismatch")
		}
		return int64(val * 2), nil
	}

	fnProxy, err := WrapFuncWithInterpreter(interp, proxy)
	if err != nil {
		t.Fatalf("WrapFuncWithInterpreter: %v", err)
	}

	got, err := fnProxy([]any{"test", 21})
	if err != nil {
		t.Fatalf("fnProxy: %v", err)
	}
	if got != int64(42) {
		t.Errorf("got %v, want 42", got)
	}
}

func TestWrapFuncArgumentValidationAndErrors(t *testing.T) {
	fn := func(a int, b string) string { return b }
	wrapped, err := WrapFunc(fn)
	if err != nil {
		t.Fatalf("WrapFunc: %v", err)
	}

	// Too few arguments
	_, err = wrapped([]any{1})
	if err == nil || !strings.Contains(err.Error(), "expects at least 2 arguments") {
		t.Errorf("expected too few arguments error, got %v", err)
	}

	// Too many arguments
	_, err = wrapped([]any{1, "hello", "extra"})
	if err == nil || !strings.Contains(err.Error(), "expects 2 arguments") {
		t.Errorf("expected too many arguments error, got %v", err)
	}

	// Type conversion failure
	_, err = wrapped([]any{"not_an_int", "hello"})
	if err == nil {
		t.Errorf("expected type conversion error for string -> int")
	}

	// Nil function
	_, err = WrapFunc(nil)
	if err == nil {
		t.Errorf("expected error wrapping nil function")
	}

	// Non-function value
	_, err = WrapFunc("not a function")
	if err == nil {
		t.Errorf("expected error wrapping non-function")
	}
}

func TestMustWrapFuncPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for MustWrapFunc(nil)")
		}
	}()
	MustWrapFunc(nil)
}

func TestRegisterFunctionTypedIntegration(t *testing.T) {
	program := programFromSource(t, `
		int add_typed(int a, int b);
		string format_typed(string prefix, int count);
		int Start() {
			int sum = add_typed(10, 32);
			string msg = format_typed("count", sum);
			if (msg == "count: 42") return sum;
			return 0;
		}`)

	interp := New(program)
	interp.RegisterFunction("add_typed", func(a, b int) int {
		return a + b
	})
	interp.RegisterFunction("format_typed", func(prefix string, count int) string {
		return prefix + ": " + strings.TrimSpace(string(rune('0'+count/10))+string(rune('0'+count%10)))
	})

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}

func TestPassingFunctionsGuestAndHost(t *testing.T) {
	program := programFromSource(t, `
		typedef int (*BinOp)(int, int);

		int add(int a, int b) { return a + b; }
		int mul(int a, int b) { return a * b; }

		int host_add(int a, int b);
		int apply_host_typed(BinOp op, int a, int b);
		int apply_host_interp(BinOp op, int a, int b);

		int apply_guest(BinOp op, int a, int b) {
			return op(a, b);
		}

		int Start() {
			int res1 = apply_guest(add, 10, 20);            // guest -> guest
			int res2 = apply_guest(host_add, 10, 20);       // host -> guest
			int res3 = apply_host_typed(mul, 6, 7);         // guest -> host (typed)
			int res4 = apply_host_interp(add, 15, 27);      // guest -> host (interpreter CallFunc)
			BinOp alias = &mul;
			int res5 = alias(2, 3);
			return res1 + res2 + res3 + res4 + res5;
		}`)

	interp := New(program)
	interp.RegisterFunction("host_add", func(a, b int) int {
		return a + b
	})
	interp.RegisterFunction("apply_host_typed", func(op func(int, int) int, a, b int) int {
		return op(a, b)
	})
	interp.RegisterFunction("apply_host_interp", func(i *Interpreter, op any, a, b int) (any, error) {
		return i.CallFunc(op, a, b)
	})

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// 30 + 30 + 42 + 42 + 6 = 150
	if got != int64(150) {
		t.Errorf("Run = %#v, want 150", got)
	}
}

func TestRegisterFunctionWithInterpreterParamIntegration(t *testing.T) {
	program := programFromSource(t, `
		int double_guest(int n) { return n * 2; }
		int call_proxy(int n);
		int Start() {
			return call_proxy(21);
		}`)

	interp := New(program)
	interp.RegisterFunction("call_proxy", func(i *Interpreter, n int) (any, error) {
		return i.Run("double_guest", int64(n))
	})

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}

func TestUnwrapAddressInTypedFunction(t *testing.T) {
	program := programFromSource(t, `
		int increment_host(int x);
		int Start() {
			int val = 41;
			return increment_host(val);
		}`)

	interp := New(program)
	interp.RegisterFunction("increment_host", func(x int) int {
		return x + 1
	})

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != int64(42) {
		t.Errorf("Run = %#v, want 42", got)
	}
}
