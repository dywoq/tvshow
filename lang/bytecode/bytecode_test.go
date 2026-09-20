package bytecode

import (
	"encoding/binary"
	"testing"

	"tvshow/lang/lexer"
	"tvshow/lang/parser"
	"tvshow/lang/token"
)

func translateSource(t *testing.T, source string) *Program {
	t.Helper()
	ast, err := parser.Parse(lexer.New("bytecode.sc", source).Tokens())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	program, err := Translate(ast)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	return program
}

func TestInstructionMarshalBinaryIsStableAndPortable(t *testing.T) {
	instruction := Instruction{
		Opcode:   JumpIfFalse,
		Operand:  42,
		Position: token.Position{Filename: "main.sc", Line: 3, Column: 12, Offset: 24},
	}
	got, err := instruction.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	// version, jump_if_false opcode, integer operand kind, 64-bit operand.
	if len(got) != 3+8+4+len("main.sc")+24 {
		t.Fatalf("encoded length = %d, want %d", len(got), 3+8+4+len("main.sc")+24)
	}
	if got[0] != 1 || got[1] != 17 || got[2] != binaryOperandInt {
		t.Fatalf("binary header = %v, want [1 17 %d]", got[:3], binaryOperandInt)
	}
	if value := int64(binary.BigEndian.Uint64(got[3:11])); value != 42 {
		t.Errorf("jump operand = %d, want 42", value)
	}
	if length := binary.BigEndian.Uint32(got[11:15]); length != uint32(len("main.sc")) {
		t.Errorf("filename length = %d, want %d", length, len("main.sc"))
	}
	if again, err := instruction.Bytes(); err != nil || string(again) != string(got) {
		t.Errorf("Bytes() = %v, %v; want MarshalBinary result", again, err)
	}
}

func TestInstructionMarshalBinaryRejectsUnknownFields(t *testing.T) {
	if _, err := (Instruction{Opcode: Opcode("unknown")}).MarshalBinary(); err == nil {
		t.Error("MarshalBinary accepted an unknown opcode")
	}
	if _, err := (Instruction{Opcode: Return, Operand: true}).MarshalBinary(); err == nil {
		t.Error("MarshalBinary accepted an unsupported operand type")
	}
}

func TestTranslateControlFlowAndAssignments(t *testing.T) {
	program := translateSource(t, `int Start(int n) { int total = 0; for (int i = 0; i < n; i++) { if (i == 3) continue; total += i; } return total; }`)
	if len(program.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(program.Functions))
	}
	fn := program.Functions[0]
	if got, want := fn.Parameters[0], "n"; got != want {
		t.Errorf("parameter = %q, want %q", got, want)
	}
	seen := map[Opcode]bool{}
	for i, instruction := range fn.Code {
		seen[instruction.Opcode] = true
		if instruction.Opcode == Jump || instruction.Opcode == JumpIfFalse || instruction.Opcode == JumpIfTrue {
			target, ok := instruction.Operand.(int)
			if !ok || target < 0 || target >= len(fn.Code) {
				t.Errorf("instruction %d has invalid jump target %#v", i, instruction.Operand)
			}
		}
	}
	for _, opcode := range []Opcode{Declare, Address, StoreIndirect, Binary, Jump, JumpIfFalse, Return} {
		if !seen[opcode] {
			t.Errorf("missing %s instruction", opcode)
		}
	}
}

func TestProgramMarshalAndUnmarshalBinaryRoundtrip(t *testing.T) {
	source := `
		int g = 10;
		int Multiply(int a, int b) {
			return a * b + g;
		}
		int Start() {
			return Multiply(2, 3);
		}`
	original := translateSource(t, source)

	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("Program.MarshalBinary: %v", err)
	}

	bytesData, err := original.Bytes()
	if err != nil || string(bytesData) != string(data) {
		t.Fatalf("Program.Bytes() does not match MarshalBinary output")
	}

	decoded, err := DecodeProgram(data)
	if err != nil {
		t.Fatalf("DecodeProgram: %v", err)
	}

	if len(decoded.Globals) != len(original.Globals) {
		t.Fatalf("decoded globals count = %d, want %d", len(decoded.Globals), len(original.Globals))
	}
	if len(decoded.Functions) != len(original.Functions) {
		t.Fatalf("decoded functions count = %d, want %d", len(decoded.Functions), len(original.Functions))
	}

	for i, origFn := range original.Functions {
		decFn := decoded.Functions[i]
		if decFn.Name != origFn.Name {
			t.Errorf("function %d name = %q, want %q", i, decFn.Name, origFn.Name)
		}
		if len(decFn.Parameters) != len(origFn.Parameters) {
			t.Errorf("function %d parameters count = %d, want %d", i, len(decFn.Parameters), len(origFn.Parameters))
		} else {
			for j, p := range origFn.Parameters {
				if decFn.Parameters[j] != p {
					t.Errorf("function %d param %d = %q, want %q", i, j, decFn.Parameters[j], p)
				}
			}
		}
		if len(decFn.Code) != len(origFn.Code) {
			t.Errorf("function %d code len = %d, want %d", i, len(decFn.Code), len(origFn.Code))
		} else {
			for j, origIns := range origFn.Code {
				decIns := decFn.Code[j]
				if decIns.Opcode != origIns.Opcode {
					t.Errorf("fn %s ins %d opcode = %v, want %v", origFn.Name, j, decIns.Opcode, origIns.Opcode)
				}
				if decIns.Operand != origIns.Operand {
					t.Errorf("fn %s ins %d operand = %#v, want %#v", origFn.Name, j, decIns.Operand, origIns.Operand)
				}
				if decIns.Position != origIns.Position {
					t.Errorf("fn %s ins %d position = %v, want %v", origFn.Name, j, decIns.Position, origIns.Position)
				}
			}
		}
	}
}

func TestProgramUnmarshalBinaryRejectsCorruptedData(t *testing.T) {
	var p Program
	if err := p.UnmarshalBinary([]byte{}); err == nil {
		t.Error("UnmarshalBinary accepted empty slice")
	}
	if err := p.UnmarshalBinary([]byte{99}); err == nil {
		t.Error("UnmarshalBinary accepted invalid version")
	}
	if _, err := DecodeProgram([]byte{1, 0, 0, 0}); err == nil {
		t.Error("DecodeProgram accepted truncated data")
	}
}

func TestTranslateCompoundLiteralsAndInitializerLists(t *testing.T) {
	source := `
		typedef struct CalculationResult { int A; int B; } CalculationResult;
		int Start() {
			CalculationResult res = (CalculationResult){ .A = 2, .B = 3 };
			int arr[3] = { 1, 2, 3 };
			return res.A + arr[1];
		}`
	program := translateSource(t, source)
	if len(program.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(program.Functions))
	}
	seen := map[Opcode]bool{}
	for _, instruction := range program.Functions[0].Code {
		seen[instruction.Opcode] = true
	}
	if !seen[MakeStruct] {
		t.Errorf("missing MakeStruct instruction in %#v", seen)
	}
	if !seen[MakeArray] {
		t.Errorf("missing MakeArray instruction in %#v", seen)
	}
}

func TestTranslateStringify(t *testing.T) {
	program := translateSource(t, `int Start() { int a = 42; string s = stringify(a); return 0; }`)
	if len(program.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(program.Functions))
	}
	foundUnaryStringify := false
	for _, ins := range program.Functions[0].Code {
		if ins.Opcode == Unary && ins.Operand == "stringify" {
			foundUnaryStringify = true
			break
		}
	}
	if !foundUnaryStringify {
		t.Errorf("missing Unary opcode with operand 'stringify'")
	}
}

func TestTranslateStringAndSizeof(t *testing.T) {
	program := translateSource(t, `string s = "hello"; int Start() { int a = sizeof(string); int b = sizeof(s); return a + b; }`)
	if len(program.Globals) == 0 {
		t.Fatalf("globals empty, expected string declaration")
	}
	if len(program.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(program.Functions))
	}
	seen := map[Opcode]bool{}
	for _, ins := range program.Functions[0].Code {
		seen[ins.Opcode] = true
	}
	if !seen[Unary] {
		t.Errorf("missing Unary opcode for sizeof(s)")
	}
	if !seen[PushLiteral] {
		t.Errorf("missing PushLiteral opcode for sizeof(string)")
	}
}

func TestTranslateShortCircuitAndGlobals(t *testing.T) {
	program := translateSource(t, `int flag = 0; int Start() { flag && (flag = 1); return flag ? flag : 2; }`)
	if len(program.Globals) == 0 || program.Globals[0].Opcode != Declare {
		t.Fatalf("globals = %#v, want declaration", program.Globals)
	}
	seen := map[Opcode]bool{}
	for _, instruction := range program.Functions[0].Code {
		seen[instruction.Opcode] = true
	}
	if !seen[ToBool] || !seen[JumpIfFalse] {
		t.Errorf("short circuit instructions not emitted: %#v", seen)
	}
}

func TestTranslateDefer(t *testing.T) {
	source := `
		void calculate() { int result = 2 + 2; }
		void start() {
			defer calculate();
		}`
	program := translateSource(t, source)
	if len(program.Functions) != 2 {
		t.Fatalf("functions = %d, want 2", len(program.Functions))
	}
	startFn := program.Functions[1]
	seenDefer := false
	for _, ins := range startFn.Code {
		if ins.Opcode == Defer {
			seenDefer = true
			if ins.Operand != 0 {
				t.Errorf("defer operand = %v, want 0", ins.Operand)
			}
		}
	}
	if !seenDefer {
		t.Errorf("missing Defer opcode in start function")
	}

	data, err := program.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	decoded, err := DecodeProgram(data)
	if err != nil {
		t.Fatalf("DecodeProgram failed: %v", err)
	}
	if len(decoded.Functions) != 2 {
		t.Fatalf("decoded functions = %d, want 2", len(decoded.Functions))
	}
}

func TestTranslateExceptionOpcodes(t *testing.T) {
	source := `
		int Divide(int a, int b) {
			if (b == 0) {
				throw "division by zero";
			}
			return a / b;
		}
		void Start() {
			try {
				int res = Divide(10, 0);
			} catch (string err) {
				string msg = err;
			}
		}`
	program := translateSource(t, source)
	if len(program.Functions) != 2 {
		t.Fatalf("functions = %d, want 2", len(program.Functions))
	}
	divideFn := program.Functions[0]
	seenDivide := map[Opcode]bool{}
	for _, ins := range divideFn.Code {
		seenDivide[ins.Opcode] = true
	}
	if !seenDivide[Throw] {
		t.Errorf("missing Throw opcode in Divide function")
	}

	startFn := program.Functions[1]
	seenStart := map[Opcode]bool{}
	for _, ins := range startFn.Code {
		seenStart[ins.Opcode] = true
	}
	for _, opcode := range []Opcode{PushCatch, PopCatch, Swap, Declare, Address, StoreIndirect} {
		if !seenStart[opcode] {
			t.Errorf("missing %s opcode in Start function", opcode)
		}
	}

	// Test binary serialization of program containing exception opcodes
	data, err := program.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	decoded, err := DecodeProgram(data)
	if err != nil {
		t.Fatalf("DecodeProgram failed: %v", err)
	}
	if len(decoded.Functions) != 2 {
		t.Fatalf("decoded functions = %d, want 2", len(decoded.Functions))
	}
}

func TestTranslateStructArrayMembers(t *testing.T) {
	source := `
		typedef struct _VECTORSTR {
			string List[];
		} VECTORSTR;
		string Start() {
			VECTORSTR Vector = (VECTORSTR){0};
			Vector.List[0] = "Hi!";
			return Vector.List[0];
		}`
	program := translateSource(t, source)
	if len(program.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(program.Functions))
	}
	seen := map[Opcode]bool{}
	for _, ins := range program.Functions[0].Code {
		seen[ins.Opcode] = true
	}
	for _, opcode := range []Opcode{AddressMember, AddressIndex, StoreIndirect, LoadIndirect} {
		if !seen[opcode] {
			t.Errorf("missing %s instruction in translation", opcode)
		}
	}
}
