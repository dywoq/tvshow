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
