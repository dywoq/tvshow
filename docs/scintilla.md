# Scintilla Specification & Documentation

## Overview

Scintilla is a light-weight embeddable interpreter inspired by standard C (ISO/IEC 9899:1999, standard C99).
It is implemented in Go under the `lang/` directory.

Scintilla source files typically use the `.sc` extension. Scintilla code is translated into a compact stack-machine bytecode format before execution, ensuring high performance and simple integration into Go applications.

### Key Concepts & Differences from C99

- **Memory Management:** Scintilla omits manual memory management (`malloc`/`free`), direct physical memory addresses, pointer arithmetic, and inline assembly. Pointer dereference (`*`) and address-of (`&`) syntax exist abstractly for variable reference and lvalue modification, but runtime memory is safely managed by the runtime environment.
- **Runtime & Host Integration:** Guest runtime functionality relies on host-registered Go functions (`Interpreter.RegisterFunction`). Guest functions and host functions can pass function references to one another and execute provided functions seamlessly.
- **Function Pointers & Aliases:** Supports referencing functions (`fn` or `&fn`), function pointer variables, function type aliases using `typedef` (e.g., `typedef int (*BinOp)(int, int);`), and strict type checking of function signatures during compile time (semantic analysis).
- **Lambdas & Closures:** Supports anonymous functions (lambdas) defined inside functions with explicit return types, closure capturing over parent function variables, and strict type alias checking.
- **Internal File Scoping:** Supports the `internal` keyword to restrict symbol visibility strictly to the declaring source file.
- **Header & Source Files:** Direct header/source compilation model simplified for interpretation.
- **Public & Modular Architecture:** Lexer, macro preprocessor, parser, semantic analyzer, bytecode translator, and interpreter are exposed as public Go packages designed for embedding, modularity, and reusability.

---

## Example Program

**main.sc**:

```c
#include "def.sc"

int Multiply(int a, int b) {
	return a * b;
}

void Start() {
	CalculationResult result;
	result.A = 2;
	result.B = 3;
	int product = Multiply(result.A, result.B);
}
```

**def.sc**:

```c
typedef struct CalculationResult {
	int A;
	int B;
} CalculationResult;
```

---

## Architecture & Execution Pipeline

Scintilla processes source code through a clean six-stage pipeline:

```
Source Code (.sc)
      │
      ▼
┌───────────┐
│   Lexer   │  (lang/lexer)  Converts raw text into a stream of tokens.
└─────┬─────┘
      │
      ▼
┌───────────┐
│   Macro   │  (lang/macro)  Expands #define, #include, conditional directives (#if, #ifdef, etc.).
└─────┬─────┘
      │
      ▼
┌───────────┐
│  Parser   │  (lang/parser) Translates tokens into an Abstract Syntax Tree (AST).
└─────┬─────┘
      │
      ▼
┌───────────┐
│ Semantic  │  (lang/semantic) Performs strict type, qualifier (const), scope, and control-flow checks.
└─────┬─────┘
      │
      ▼
┌───────────┐
│ Bytecode  │  (lang/bytecode) Lowers valid AST into stack-machine instructions.
└─────┬─────┘
      │
      ▼
┌───────────┐
│Interpreter│  (lang/interpreter) Executes bytecode instructions & handles host Go calls.
└───────────┘
```

---

## Language Components & Go API

The codebase is organized under `lang/` into seven distinct Go packages:

### 1. `lang/token`

Defines lexical tokens, source positions (`Position`), keywords, and operators.

- **Key Types & Functions:**
  - `Position`: Represents `Filename`, `Line`, `Column`, and `Offset`.
  - `Token`: Combines `Type`, `Literal` string, and `Pos`.
  - `LookupIdent(literal string)`: Determines if an identifier is a C99 keyword.
  - `RegisterKeyword(name, tok, stringRepr)`: Extends the keyword table dynamically.

### 2. `lang/lexer`

Performs lexical analysis on Scintilla source text.

- **Features:**
  - Standard C99 tokenization: Identifiers, Integer literals (decimal, octal `077`, hex `0xFF`), Floating-point literals (`3.14`, `1e-10`), Character (`'a'`), String (`"hello"`).
  - Suffix support: Integer (`u`, `l`) and Float (`f`, `l`) suffixes.
  - Skips whitespace and single-line (`//`) and block (`/* ... */`) comments.
- **Key Functions:**
  - `New(filename, input string) *Lexer`
  - `NextToken() token.Token`
  - `Tokens() []token.Token`

### 3. `lang/macro`

Token-based preprocessor supporting C99 preprocessor directives and macro expansion.

- **Supported Directives & Features:**
  - Macros: Object-like (`#define FOO 1`) and Function-like (`#define ADD(a, b) ((a)+(b))`).
  - Variadic Macros: Support for `...` and `__VA_ARGS__`.
  - Macro Operators: Stringification (`#`) and Token Pasting (`##`).
  - Undefining: `#undef NAME`.
  - File Inclusion: `#include "file.sc"` via a user-defined `IncludeResolver`.
  - Conditionals: `#if`, `#ifdef`, `#ifndef`, `#elif`, `#else`, `#endif`, and the `defined(NAME)` operator.
  - Infinite expansion prevention via `MaxExpansion` limit.
- **Key Functions:**
  - `New() *Expander`
  - `(e *Expander) Define(m Macro)`
  - `(e *Expander) Undef(name string)`
  - `(e *Expander) Expand(input []token.Token) ([]token.Token, error)`

### 4. `lang/parser`

Constructs an AST from preprocessed tokens.

- **AST Node Types (`Node`, `Expression`, `Statement`, `Declaration`):**
  - Declarations: `VarDecl`, `FunctionDecl`, `StaticAssertDecl`, `TypeSpec` (`struct`, `union`, `enum`, `typedef`).
  - Statements: `BlockStmt`, `ExprStmt`, `IfStmt`, `SwitchStmt`, `CaseStmt`, `WhileStmt`, `DoWhileStmt`, `ForStmt`, `JumpStmt` (`break`, `continue`, `return`, `goto`), `LabelStmt`, `ThrowStmt`, `TryCatchStmt`, `DeferStmt`.
  - Expressions: `IdentExpr`, `LiteralExpr`, `UnaryExpr`, `BinaryExpr`, `AssignExpr`, `ConditionalExpr` (`?:`), `CallExpr`, `IndexExpr` (`[]`), `MemberExpr` (`.` and `->`), `CastExpr`, `SizeofExpr`, `StringifyExpr`, `CommaExpr`, `CompoundLiteralExpr`, `InitializerListExpr`.
- **Key Functions:**
  - `Parse(tokens []token.Token) (*Program, error)`

### 5. `lang/semantic`

Validates AST semantics, types, and qualifiers prior to bytecode translation.

- **Validation Checks:**
  - **Qualifier Checks (`const`):** Enforces immutability for `const` variables, `const` struct fields, fields of `const` struct instances, elements of `const` arrays, and target values of pointers-to-const. Rejects assignments (`=`, `+=`, etc.) and modifications (`++`, `--`) targeting `const` lvalues. Requires initializers for local `const` variables.
  - **Array Checks:** Verifies subscripted expressions are arrays, pointers, or strings and subscript indices are integers. Enforces non-negative array bounds, rejects arrays with `void` element types, and validates initializer list bounds against fixed array sizes.
  - **Struct & Union Field Checks:** Disallows fields of type `void`, checks for duplicate member names within aggregate declarations, verifies member access operators (`.` on structs/unions vs `->` on struct/union pointers), and validates member existence.
  - **Function Parameters & Call Checks:** Rejects named parameters declared with `void` type, enforces exact argument count and parameter type compatibility for function calls, verifies strict function signature compatibility for function aliases and function pointers, and verifies called expressions are callable objects.
  - **Local Variables & Declarations:** Rejects variables declared with `void` type, enforces type compatibility between variable declarations and initializers, and tracks scope/identifier symbol tables.
  - **Return Statements:** Enforces that `void` functions do not return values, non-void functions return a value, and return expression types match function return types.
  - **Expression & Operator Type Checks:** Enforces operand constraints for bitwise operators (integers only), unary operators, binary arithmetic, switch statement quantities, and condition expressions.
  - **Control Flow & Scopes:** Validates `break`/`continue` within loops/switches, `case`/`default` inside switch statements, and `goto` label targets.
  - **Exception Checks:** Validates that `throw` expressions evaluate to string exceptions. Non-string types (integers, floats, structs, arrays, etc.) are rejected at compile time. Validates that `catch` block parameter types are `string`.
- **Key Functions:**
  - `Analyze(program *parser.Program) error`

### 6. `lang/bytecode`

Translates semantically validated AST into stack-machine instructions. Runs semantic analysis automatically during translation.

- **Key Features:**
  - Lowers high-level control flow (`if`, `while`, `for`, `switch`, `goto`, `break`, `continue`) to resolved zero-based jump instruction indices.
  - Retains left-to-right short-circuit evaluation for `&&`, `||`, and `?:`.
  - Retains expression evaluation ordering and postfix increment/decrement semantics.
  - Produces binary-encodable bytecode.
- **Key Functions:**
  - `Translate(program *parser.Program) (*Program, error)`
  - `(p *Program) MarshalBinary() ([]byte, error)` / `(p *Program) Bytes() ([]byte, error)`
  - `(p *Program) UnmarshalBinary(data []byte) error`
  - `DecodeProgram(data []byte) (*Program, error)`
  - `(i Instruction) MarshalBinary() ([]byte, error)` / `(i Instruction) Bytes() ([]byte, error)`
  - `(i *Instruction) UnmarshalBinary(data []byte) error`
  - `DecodeInstruction(data []byte) (Instruction, error)`

### 7. `lang/interpreter`

Stack-based virtual machine executing Scintilla bytecode programs.

- **Key Features:**
  - Executes bytecode instructions (`Execute`, `Run`).
  - Maintained variable scopes (`globals` and call stack frames).
  - Interoperability with host Go code via `RegisterFunction(name, fn)`.
  - Built-in position functions (`line()`, `column()`, `filename()`) returning current source code position.
  - Native exception system supporting guest `try ... catch` blocks and host-level uncaught exception handlers (`SetExceptionHandler`).
  - Built-in defer mechanism executing specified functions upon function exit (return, completion, or exception) in LIFO order.
  - Binary instruction decoding and execution (`ExecuteBinary`, `DecodeInstruction`).
  - Built-in interactive debugger (`Debugger`) providing breakpoints, stepping modes, execution control, and runtime state inspection.
- **Key Functions & Types:**
  - `New(program ...*bytecode.Program) *Interpreter`
  - `NewFromBinary(programData []byte) (*Interpreter, error)`
  - `(i *Interpreter) RegisterFunction(name string, fn Function)`
  - `(i *Interpreter) Run(name string, args ...any) (any, error)`
  - `(i *Interpreter) Execute(code []bytecode.Instruction) (any, error)`
  - `(i *Interpreter) ExecuteBinary(code [][]byte) (any, error)`
  - `(i *Interpreter) ToFunction(v any) (Function, error)`
  - `(i *Interpreter) CallFunc(fn any, args ...any) (any, error)`
  - `(i *Interpreter) SetExceptionHandler(handler ExceptionHandler)`
  - `(i *Interpreter) SetDebugger(d *Debugger)`
  - `InterpretBinary(code [][]byte) (any, error)`
  - `InterpretProgramBinary(programData []byte, funcName string, args ...any) (any, error)`
  - `NewDebugger() *Debugger`
  - `(d *Debugger) Attach(interp *Interpreter)`
  - `(d *Debugger) SetHook(fn HookFunc)`
  - `(d *Debugger) SetBreakpoint(filename string, line int) *Breakpoint`
  - `(d *Debugger) SetFunctionBreakpoint(funcName string) *Breakpoint`
  - `(d *Debugger) StepInto()`, `StepOver()`, `StepOut()`, `StepInstruction()`, `Continue()`, `Pause()`, `Stop()`
  - `(d *Debugger) CallStack() []StackFrame`
  - `(d *Debugger) GetVariable(name string) (any, bool)`, `SetVariable(name string, value any) bool`

---

## Bytecode & Binary Specification

### Instruction Opcodes

| Opcode           | Operand                   | Effect / Description                                                                                                                            |
| ---------------- | ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `declare`        | `string` (var name)       | Creates a variable cell in the current execution frame.                                                                                         |
| `push_literal`   | `string` (raw literal)    | Parses and pushes an integer, floating-point, character, or string literal.                                                                     |
| `load`           | `string` (var/func name)  | Pushes the value of a variable or a function reference.                                                                                         |
| `address`        | `string` (var name)       | Pushes an address handle referencing a named variable.                                                                                          |
| `address_index`  | none                      | Pops `index` and base array `address`/slice, pushes an address handle for `base[index]`.                                                        |
| `address_member` | `string` (member)         | Pops aggregate `address`, pushes an address handle for `aggregate.member`.                                                                      |
| `load_indirect`  | none                      | Pops an address handle and pushes its contained value.                                                                                          |
| `store_indirect` | none                      | Pops value and address handle, stores value to address, and leaves value on stack.                                                              |
| `unary`          | `string` (op symbol)      | Applies unary operation (`!`, `~`, `+`, `-`).                                                                                                   |
| `to_bool`        | none                      | Pops value and pushes boolean truth value `0` or `1`.                                                                                           |
| `binary`         | `string` (op symbol)      | Pops right and left operands, applies binary operation (`+`, `-`, `*`, `/`, `%`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `&`, `\|`, `^`, `<<`, `>>`). |
| `dup`            | none                      | Duplicates top value on stack.                                                                                                                  |
| `rotate`         | none                      | Rotates top 3 stack values (`a, b, c` -> `b, a, c`); preserves old value in postfix updates.                                                    |
| `pop`            | none                      | Discards top value from stack.                                                                                                                  |
| `call`           | `int` (arg count)         | Pops argument values and target function, executes call, pushes result.                                                                         |
| `jump`           | `int` (instruction index) | Unconditional jump to target instruction index.                                                                                                 |
| `jump_if_false`  | `int` (instruction index) | Pops condition; jumps if false (`0`).                                                                                                           |
| `jump_if_true`   | `int` (instruction index) | Pops condition; jumps if true (non-zero).                                                                                                       |
| `return`         | none                      | Returns current stack top value (or `nil` if stack is empty).                                                                                   |
| `make_array`     | `int` (array length)      | Pushes a new `[]any` slice of specified initial length onto the stack.                                                                          |
| `make_struct`    | none                      | Pushes a new `map[string]any` map onto the stack.                                                                                               |
| `convert`        | `string` (target type)    | Pops value, converts value to specified target type (or reports a type conversion error if types do not match), and pushes result onto stack.   |
| `throw`          | none                      | Pops exception string from stack and initiates exception unwinding.                                                                             |
| `push_catch`     | `int` (instruction index) | Registers a catch handler target for the current execution frame.                                                                               |
| `pop_catch`      | none                      | Removes the top catch handler from the current execution frame.                                                                                 |
| `swap`           | none                      | Swaps top two values on the operand stack.                                                                                                      |
| `defer`          | `int` (arg count)         | Pops argument values and target function, registering deferred function call on current execution frame.                                        |

---

## Lambdas (Anonymous Functions)

Scintilla supports lambdas, which are anonymous functions defined inside guest functions.

### 1. Rules & Syntax

- **Syntax:** `ReturnType(Parameters) { Body }`
- **Function Context Requirement:** Lambdas must be defined within a function (disallowed at global scope).
- **Return Type Requirement:** Lambdas must explicitly define their return type (e.g., `int`, `void`, `float`, `string`, `auto`, etc.).
- **Lexical Closures:** Lambdas can access and modify any local variables that exist in their parent enclosing function at runtime.
- **Type Checking & Aliases:** Lambdas are assigned function signature types and strictly verified against function type aliases (`typedef`) or target variable types during semantic analysis.

### 2. Example Program

```c
typedef int (*BinOp)(int, int);

void start() {
	int result = 2 * 2;

	// int is the return type and (int given_result) contains arguments
	auto lambda = int(int given_result) {
		return given_result * 2 + result;
	};

	int res = lambda(result);

	// Satisfies function type alias BinOp
	BinOp op = int(int a, int b) { return a + b; };

	// Rejected at compile-time: return type int does not match UserStruct
	// UserStruct user = lambda(result);
}
```

---

## File-Scoped Visibility (`internal`)

Scintilla supports the `internal` keyword to limit the visibility of symbols (functions, variables, typedefs, etc.) strictly to the source file where they are declared.

### 1. Rules & Syntax

- **Visibility:** A symbol declared with `internal` can only be referenced by code within the same source file.
- **Cross-File Access Error:** Attempting to access an `internal` symbol from another source file (e.g. via `#include`) produces a compile-time semantic analysis error.
- **File Name Tracking:** The macro preprocessor propagates source file locations (`Pos.Filename`) into `internal` keyword tokens so file boundary rules are strictly enforced regardless of header inclusions.

### 2. Example Program

**def.sc**:

```c
internal float PI = 3.14;
```

**main.sc**:

```c
#include "def.sc"

float PI2 = PI; // Semantic analysis error: cannot access internal symbol "PI" from file "main.sc"
```

---

## Function References, Aliases & Interoperability

Scintilla provides full support for referencing functions, function aliases, and bidirectional invocation between guest Scintilla code and host Go code.

### 1. Function References & Aliases in Guest Code

Functions can be referenced by name (`fn`) or address (`&fn`), stored in variables, passed as arguments, or stored in struct fields. Function aliases are declared using standard C `typedef` syntax.

```c
// Define a function type alias for a function taking two ints and returning an int
typedef int (*BinaryOp)(int, int);

int Add(int a, int b) { return a + b; }
int Multiply(int a, int b) { return a * b; }

// Guest function taking a function alias parameter
int ExecuteOp(BinaryOp op, int x, int y) {
    return op(x, y); // Direct call or (*op)(x, y)
}

int Start() {
    BinaryOp op = Add;
    int sum = ExecuteOp(op, 10, 20);      // 30
    int prod = ExecuteOp(&Multiply, 3, 4); // 12
    return sum + prod;                    // 42
}
```

### 2. Strict Type Checking

The semantic analyzer (`lang/semantic`) enforces strict type safety for function calls and assignments:

- **Signature Matching:** Variable initializations, assignments, and function parameters expecting function pointers require matching return types, argument counts, and argument types.
- **Callable Verification:** Calling non-function types (e.g. `int x = 5; x()`) is rejected with a compile-time error (`called object of type "int" is not a function`).
- **Argument & Return Validation:** Calls on function pointers/aliases check parameter count and argument types against the function pointer signature.

### 3. Host and Guest Interoperability

Guest function references can be passed directly to host Go functions, and host function references can be passed to guest code.

- **Passing Guest Functions to Host Go Functions:**
  When host functions registered via `RegisterFunction` accept a Go function parameter (e.g. `func(op func(int, int) int, a, b int) int` or `interpreter.Function` or `any`), Scintilla automatically wraps guest function references into callable Go functions.
  Host functions can also use `interp.ToFunction(v)` or `interp.CallFunc(v, args...)` to invoke guest function references directly.

- **Passing Host Functions to Guest Functions:**
  Host functions registered with `RegisterFunction` can be passed to guest functions expecting function pointers/aliases and invoked seamlessly from guest code.

```go
// Register host proxy function accepting a guest function parameter
interp.RegisterFunction("ApplyHost", func(op func(int, int) int, a, b int) int {
    return op(a, b) // Executes guest function from Go host
})
```

---

## Defer Statement (`defer`)

Scintilla provides native builtin support for the `defer` statement.

### 1. Rules & Syntax

- **Function Call Requirement:** A `defer` statement must be followed by a function call expression (e.g. `defer calculate();` or `defer print(msg);`). Passing non-function-call statements or variable declarations (such as `defer int result = 2 + 2;`) produces a compile-time parsing error.
- **Execution Lifecycle:** Deferred calls are executed when the enclosing function returns, completes, or exits due to a thrown exception.
- **LIFO Execution Order:** If multiple `defer` statements execute in a function, their calls are deferred onto a stack and executed in Last-In, First-Out (LIFO) order when the function exits.
- **Argument Evaluation:** Arguments passed to a deferred call are evaluated at the time the `defer` statement is reached during execution.

### 2. Example Program

```c
void calculate() {
	int result = 2 + 2;
}

void start() {
	defer calculate();
	// This shall cause parsing error, because this is not a function call:
	// defer int result = 2 + 2;
}
```

---

## Exception Handling (`try`, `catch`, `throw`)

Scintilla provides native bytecode-level exception handling using `throw`, `try`, and `catch` constructs.

### 1. Rules & Syntax

- **String Exceptions Only:** Exceptions in Scintilla are string values. Throwing primitive non-string types (`int`, `float`, `char`, etc.) or user-defined types (`struct`) is rejected by semantic analysis at compile time.
- **Catch Clause:** The `catch` statement accepts a `string` parameter (e.g. `catch (string exception)`).
- **Execution Unwinding:** Executing `throw` terminates execution of the current statement sequence or function, unwinding call frames until a matching guest `try ... catch` block is found. If no guest catch block catches the exception, host code's exception handler is invoked.

### 2. Example Program

```c
int Divide(int A, int B) {
      if (B == 0) {
            throw "division by zero is not allowed";
            // another return here is extra.
      }
      return A / B;
}

void Start() {
     try {
            int Result = Divide(10, 0);
            // Do something with result
     } catch (string exception) {
            // Do something with exception
     }
}
```

### 3. Host Exception Handler (`SetExceptionHandler`)

Host code can register an exception handler to receive full exception details (`ExceptionInfo`) when guest code produces an uncaught exception:

```go
interp := interpreter.New(program)
interp.SetExceptionHandler(func(info interpreter.ExceptionInfo) {
    fmt.Printf("Uncaught exception: %s in function %s at %s\n",
        info.Message, info.FuncName, info.Position)
})

result, err := interp.Run("Start")
```

`ExceptionInfo` contains:

- `Message`: The thrown exception string.
- `Position`: The source location (`token.Position`) where `throw` occurred.
- `FuncName`: The function name where `throw` occurred.
- `CallStack`: The slice of active stack frames (`[]StackFrame`) captured at throw time.

---

## Built-in Source Position Functions (`line()`, `column()`, `filename()`)

Scintilla provides three built-in functions in guest code to retrieve the current source code position at runtime. These functions are built-in by the interpreter and cannot be overridden or removed externally. They are primarily used for printing debugging messages or logging in guest programs.

### Functions

- `int line()`: Returns the 1-based source code line number of the current call site.
- `int column()`: Returns the 1-based source code column number of the current call site.
- `string filename()`: Returns the filename string of the current source file.

### Example Usage

```c
void LogMessage(string message) {
    printf("%s:%d:%d: %s\n", filename(), line(), column(), message);
}

int Start() {
    int currentLine = line();
    int currentColumn = column();
    string currentFile = filename();
    return currentLine;
}
```

---

## Interpreter Debugger

The `lang/interpreter` package includes a full-featured Debugger system for inspecting and controlling Scintilla bytecode execution.

### Capabilities

1. **Breakpoints:**
   - **Line Breakpoints:** Triggered when execution reaches a specific file line (`SetBreakpoint(filename, line)`).
   - **Function Breakpoints:** Triggered at the entry instruction of a target function (`SetFunctionBreakpoint(funcName)`).
   - **Breakpoint Management:** Supports clearing specific breakpoints (`ClearBreakpoint(id)`, `ClearBreakpointAt(filename, line)`, `ClearFunctionBreakpoint(funcName)`) or clearing all breakpoints (`ClearAllBreakpoints()`).

2. **Stepping Modes (`DebugAction`):**
   - `Continue`: Resumes execution until the next breakpoint or pause request.
   - `StepInto`: Steps to the next line or into a function call.
   - `StepOver`: Steps to the next line in the current function, executing function calls inline.
   - `StepOut`: Runs until the current function returns to its caller.
   - `StepInstruction`: Steps a single bytecode instruction regardless of line boundaries.
   - `Stop`: Terminates VM execution immediately with `ErrStopped`.

3. **Runtime State Inspection & Modification:**
   - **Call Stack:** `CallStack()` returns active call frames (`StackFrame`) including function names, source positions, program counters, and local scopes. `CurrentFrame()` returns the innermost frame.
   - **Variables:** `Locals()` returns current frame variables; `Globals()` returns global variables. `GetVariable(name)` and `SetVariable(name, value)` dynamically read or modify local and global variables during debug hooks.
   - **Stack & Instructions:** `OperandStack()` returns a copy of the VM value stack; `CurrentInstruction()` returns the opcode currently being executed.

### Usage Example

```go
interp := interpreter.New(program)
dbg := interpreter.NewDebugger()
dbg.Attach(interp)

dbg.SetFunctionBreakpoint("Multiply")

dbg.SetHook(func(d *interpreter.Debugger, event interpreter.DebugEvent) interpreter.DebugAction {
    fmt.Printf("Hit debug event %s in %s at line %d\n", event.Kind, event.FuncName, event.Position.Line)
    if val, ok := d.GetVariable("a"); ok {
        fmt.Printf("Variable a = %v\n", val)
    }
    return interpreter.ActionStepOver
})

result, err := interp.Run("Start")
```

---

### Binary Format Specification (`Instruction.MarshalBinary` & `Program.MarshalBinary`)

Bytecode structures can be serialized into portable, architecture-independent binary streams (`Version 1` layout) and reversed back into full Go structs (`UnmarshalBinary`, `DecodeProgram`, `DecodeInstruction`):

#### 1. Instruction Layout (`Instruction.MarshalBinary`)

1. **Header (3 bytes):**
   - Byte 0: Instruction Format Version (`1`)
   - Byte 1: Opcode numeric identifier (1-27)
   - Byte 2: Operand Kind (`0` = None, `1` = String, `2` = Integer)
2. **Operand Payload (variable):**
   - Kind `0`: 0 bytes.
   - Kind `1`: `uint32` Big-Endian string length + UTF-8 bytes.
   - Kind `2`: `int64` Big-Endian signed integer value.
3. **Source Position Payload:**
   - Filename: `uint32` Big-Endian length + UTF-8 bytes.
   - Line: `int64` Big-Endian signed integer.
   - Column: `int64` Big-Endian signed integer.
   - Offset: `int64` Big-Endian signed integer.

#### 2. Whole Program Layout (`Program.MarshalBinary`)

1. **Header (1 byte):**
   - Byte 0: Program Format Version (`1`)
2. **Globals Section:**
   - `uint32` Big-Endian count of global instructions.
   - Sequence of global instructions, each encoded as a `uint32` Big-Endian byte-length followed by the serialized instruction binary payload.
3. **Functions Section:**
   - `uint32` Big-Endian count of translated functions.
   - For each function:
     - Function Name: `uint32` Big-Endian length + UTF-8 bytes.
     - Parameters: `uint32` Big-Endian count + array of parameter names (each `uint32` length + UTF-8 bytes).
     - Code: `uint32` Big-Endian instruction count + sequence of function body instructions (each `uint32` length + instruction payload).

---

## Detailed C99 Compliance Matrix

Scintilla aims for high alignment with ISO/IEC 9899:1999 (C99) syntax and semantics while maintaining a lightweight runtime execution model.

### 1. Preprocessor Compliance

| C99 Preprocessor Feature                                              | Scintilla Status            | Details & Notes                                                                 |
| --------------------------------------------------------------------- | --------------------------- | ------------------------------------------------------------------------------- |
| `#define` (Object-like)                                               | **Supported**               | Full macro definition and expansion.                                            |
| `#define` (Function-like)                                             | **Supported**               | Parameterized macro expansion with argument substitution.                       |
| Variadic Macros (`...`, `__VA_ARGS__`)                                | **Supported**               | Variadic arguments in function-like macros.                                     |
| Stringification (`#`) & Token Pasting (`##`)                          | **Supported**               | Converts tokens to string literals (`#`) and pastes tokens (`##`).              |
| `#undef`                                                              | **Supported**               | Removes macro definition.                                                       |
| `#include`                                                            | **Supported**               | Resolver-backed custom header inclusion via `IncludeResolver`.                  |
| Conditionals (`#if`, `#ifdef`, `#ifndef`, `#elif`, `#else`, `#endif`) | **Supported**               | Full conditional compilation evaluation.                                        |
| `defined` Operator                                                    | **Supported**               | Evaluated during `#if`/`#elif` expansion (`defined(X)` or `defined X`).         |
| `#error`                                                              | **Supported**               | Signals a compilation/preprocessor error with an optional message.              |
| `#line`, `#pragma`                                                    | **Removed / Not Supported** | Preprocessor directives `#line` and `#pragma` are not supported.                |
| Predefined Macros (`__LINE__`, `__FILE__`, etc.)                      | **Removed / Not Supported** | Standard predefined macros such as `__LINE__` and `__FILE__` are not supported. |

### 2. Lexical & Language Syntax Compliance

| C99 Syntax Feature          | Scintilla Status  | Details & Notes                                                                                                                                                                                                                                                                                                                                                                                                        |
| --------------------------- | ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Keywords                    | **Supported**     | C99 keywords recognized, except for removed keywords (`register`, `volatile`, `restrict`, `static`, `extern`, `inline`) (`auto`, `break`, `case`, `char`, `const`, `continue`, `default`, `defer`, `do`, `double`, `else`, `enum`, `float`, `for`, `goto`, `if`, `internal`, `int`, `long`, `return`, `short`, `signed`, `sizeof`, `stringify`, `struct`, `switch`, `typedef`, `union`, `unsigned`, `void`, `while`, `_Bool`, `_Complex`, `_Imaginary`). |
| Comments                    | **Supported**     | Line comments (`//`) and block comments (`/* ... */`).                                                                                                                                                                                                                                                                                                                                                                 |
| Numeric Literals            | **Supported**     | Decimal, Hexadecimal (`0x`), Octal (`0`), Floating-point scientific notation (`1e-10`), suffixes (`u`, `l`, `f`).                                                                                                                                                                                                                                                                                                      |
| Character & String Literals | **Supported**     | Escaped sequences handled by lexer/interpreter.                                                                                                                                                                                                                                                                                                                                                                        |
| Trigraphs / Digraphs        | _Not Implemented_ | Alternative token representations are not supported.                                                                                                                                                                                                                                                                                                                                                                   |

### 3. Statements & Control Flow

| C99 Statement Feature                           | Scintilla Status | Details & Notes                                                                                                                                                                       |
| ----------------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Selection (`if`, `if`-`else`)                   | **Supported**    | Translated to conditional jumps.                                                                                                                                                      |
| `switch`, `case`, `default`                     | **Supported**    | Full switch statement lowerings with fall-through support, integer quantity checks, and case label validation.                                                                        |
| Iteration (`while`, `do`-`while`)               | **Supported**    | Translated to jump constructs.                                                                                                                                                        |
| `for` Loops                                     | **Supported**    | Includes support for C99 loop-header variable declarations (`for (int i = 0; ...)`).                                                                                                  |
| Jump Statements (`break`, `continue`, `return`) | **Supported**    | Validated during semantic pass for enclosing loop/switch scopes, void vs non-void returns, and return expression type compatibility.                                                  |
| Exception Statements (`throw`, `try`-`catch`)   | **Supported**    | Native bytecode-level exception handling. Semantic analyzer verifies string exception types for throw expressions and catch parameters. Host code can register `SetExceptionHandler`. |
| Defer Statement (`defer`)                       | **Supported**    | Builtin feature executing deferred function calls upon function exit or exception in LIFO order. |
| `goto` & Labeled Statements                     | **Supported**    | Resolved to instruction jump targets in function context.                                                                                                                             |
| `return`                                        | **Supported**    | Enforces void vs non-void function return value rules and type compatibility.                                                                                                         |

### 4. Declarations & Types

| C99 Type / Declaration Feature                                             | Scintilla Status                    | Details & Notes                                                                                                                                                                                                                                                                                                                                                                                                          |
| -------------------------------------------------------------------------- | ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Primitive Types (`int`, `float`, `char`, `string`, `double`, `void`, etc.) | **Supported**                       | Lexed, parsed, semantically validated, and evaluated at runtime using Go dynamic representations (`int64`, `float64`, `string`). Includes native `string` type. `void` checked for invalid variable, field, or parameter declarations.                                                                                                                                                                                   |
| Structs (`struct`) & Unions (`union`)                                      | **Supported**                       | Member declaration parsing, duplicate member validation, void member rejection, member access operator validation (`.` vs `->`), and runtime `map[string]any` field access.                                                                                                                                                                                                                                              |
| Enumerations (`enum`)                                                      | **Supported**                       | Enumerator constants registered in value symbol scope.                                                                                                                                                                                                                                                                                                                                                                   |
| Typedefs (`typedef`)                                                       | **Supported**                       | Custom type identifiers tracked in parser and semantic scopes, including function pointer and function signature aliases (`typedef int (*BinOp)(int, int)`).                                                                                                                                                                                                                                                             |
| Array Declarations                                                         | **Supported**                       | Fixed and variable array declarator suffixes parsed; semantic analyzer enforces integer subscripts, non-negative array bounds, non-void element types, and initializer list bounds checking.                                                                                                                                                                                                                             |
| Specifiers (`const`, `auto`, `internal`)                                   | **Supported** (for `const`, `auto`, `internal`) | `const` qualifiers strictly enforced by semantic analyzer. `auto` represents a dynamic type. `internal` specifier limits symbol visibility to declaring source file. Note: `static`, `extern`, `inline`, `register`, `volatile`, and `restrict` keywords removed. |
| Standard Library (`<stdio.h>`, `<stdlib.h>`, etc.)                         | _Divergent_                         | Standard C library headers are omitted. Native host functions are exposed via Go bindings (`RegisterFunction`).                                                                                                                                                                                                                                                                                                          |

### 5. Expressions & Operators

| C99 Expression Feature                | Scintilla Status | Details & Notes                                                                                                                                                                                  |
| ------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Primary Expressions & Identifiers     | **Supported**    | Identifiers, constants, string literals, parenthesized expressions.                                                                                                                              |
| Postfix / Prefix (`++`, `--`)         | **Supported**    | Address-based update semantics (`Rotate` opcode preserves postfix value); semantic analyzer rejects modification of `const` lvalues.                                                             |
| Unary Operators (`+`, `-`, `!`, `~`)  | **Supported**    | Integer and floating-point unary evaluation with operand type checks.                                                                                                                            |
| Address-Of (`&`) & Dereference (`*`)  | **Supported**    | Syntax and lvalue address resolution supported; dereferencing pointers to `const` produces read-only lvalues.                                                                                    |
| Binary Arithmetic & Bitwise Operators | **Supported**    | `+`, `-`, `*`, `/`, `%`, `&`, `\|`, `^`, `<<`, `>>`. Bitwise operators strictly restricted to integer operands.                                                                                  |
| Relational & Equality Operators       | **Supported**    | `<`, `>`, `<=`, `>=`, `==`, `!=` with type compatibility checking.                                                                                                                               |
| Logical Operators (`&&`, `\|\|`)      | **Supported**    | Short-circuit evaluation retained via conditional jumps.                                                                                                                                         |
| Conditional Operator (`?:`)           | **Supported**    | Short-circuit ternary evaluation retained via conditional jumps with branch type compatibility checking.                                                                                         |
| Assignment & Compound Assignment      | **Supported**    | `=`, `+=`, `-=`, `*=`, `/=`, `%=`, `&=`, `\|=`, `^=`, `<<=`, `>>=`. Strict lvalue and `const` immutability enforcement.                                                                          |
| Comma Expression (`,`)                | **Supported**    | Sequential evaluation yielding last expression result.                                                                                                                                           |
| Function Calls & Lambdas              | **Supported**    | Argument count and parameter type compatibility checking for guest and host function invocations, including first-class function pointer, function alias, and lambda anonymous function calls (`fn(a, b)` or `(*fn)(a, b)`). |
| Cast Expressions (`(type)expr`)       | **Supported**    | Parsed, type-checked during semantic analysis, and evaluated at runtime. Converts `auto` variables to specified target types, reporting a type conversion error if types do not match.           |
| `sizeof` Operator                     | **Supported**    | Evaluates byte size for type specifiers (including fixed-size arrays) or element count/length for dynamic/fixed array and string expressions.                                                    |
| `stringify()` Operator                | **Supported**    | Converts any value, struct, array, etc. into a value having type `string`.                                                                                                                       |
| Compound Literals & Initializer Lists | **Supported**    | Full parsing, semantic analysis, bytecode lowering, and VM execution for struct/array compound literals and designated initializer lists.                                                        |
| `_Static_assert`                      | **Parsed Only**  | Syntactically parsed in AST; semantic analyzer evaluates condition without compile-time termination.                                                                                             |
