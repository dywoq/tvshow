# Scintilla Specification & Documentation

## Overview

Scintilla is a light-weight embeddable interpreter inspired by standard C (ISO/IEC 9899:1999, standard C99).
It is implemented in Go under the `lang/` directory.

Scintilla source files typically use the `.sc` extension. Scintilla code is translated into a compact stack-machine bytecode format before execution, ensuring high performance and simple integration into Go applications.

### Key Concepts & Differences from C99

- **Memory Management:** Scintilla omits manual memory management (`malloc`/`free`), direct physical memory addresses, pointer arithmetic, and inline assembly. Pointer dereference (`*`) and address-of (`&`) syntax exist abstractly for variable reference and lvalue modification, but runtime memory is safely managed by the runtime environment.
- **Runtime & Host Integration:** Guest runtime functionality relies on host-registered Go functions (`Interpreter.RegisterFunction`).
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
│ Semantic  │  (lang/semantic) Validates identifiers, scopes, lvalues, break/continue/switch flow.
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
  - Statements: `BlockStmt`, `ExprStmt`, `IfStmt`, `SwitchStmt`, `CaseStmt`, `WhileStmt`, `DoWhileStmt`, `ForStmt`, `JumpStmt` (`break`, `continue`, `return`, `goto`), `LabelStmt`.
  - Expressions: `IdentExpr`, `LiteralExpr`, `UnaryExpr`, `BinaryExpr`, `AssignExpr`, `ConditionalExpr` (`?:`), `CallExpr`, `IndexExpr` (`[]`), `MemberExpr` (`.` and `->`), `CastExpr`, `SizeofExpr`, `CommaExpr`, `CompoundLiteralExpr`, `InitializerListExpr`.
- **Key Functions:**
  - `Parse(tokens []token.Token) (*Program, error)`

### 5. `lang/semantic`
Validates AST semantics prior to bytecode translation.
- **Validation Checks:**
  - Identifier resolution and scope tracking (separate symbol tables for values and types).
  - Function predeclarations and redefinition detection.
  - Lvalue assignability verification for assignment targets (`=`, `+=`, etc.).
  - Control-flow validity: `break` and `continue` inside loops, `break` inside switches, `case`/`default` inside switch statements, and `goto` target label resolution within functions.
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

### 7. `lang/interpreter`
Stack-based virtual machine executing Scintilla bytecode programs.
- **Key Features:**
  - Executes bytecode instructions (`Execute`, `Run`).
  - Maintained variable scopes (`globals` and call stack frames).
  - Interoperability with host Go code via `RegisterFunction(name, fn)`.
  - Binary instruction decoding and execution (`ExecuteBinary`, `DecodeInstruction`).
- **Key Functions:**
  - `New(program ...*bytecode.Program) *Interpreter`
  - `(i *Interpreter) RegisterFunction(name string, fn Function)`
  - `(i *Interpreter) Run(name string, args ...any) (any, error)`
  - `(i *Interpreter) Execute(code []bytecode.Instruction) (any, error)`
  - `InterpretBinary(code [][]byte) (any, error)`

---

## Bytecode & Binary Specification

### Instruction Opcodes

| Opcode | Operand | Effect / Description |
| --- | --- | --- |
| `declare` | `string` (var name) | Creates a variable cell in the current execution frame. |
| `push_literal` | `string` (raw literal) | Parses and pushes an integer, floating-point, character, or string literal. |
| `load` | `string` (var/func name) | Pushes the value of a variable or a function reference. |
| `address` | `string` (var name) | Pushes an address handle referencing a named variable. |
| `address_index` | none | Pops `index` and base array `address`/slice, pushes an address handle for `base[index]`. |
| `address_member` | `string` (member) | Pops aggregate `address`, pushes an address handle for `aggregate.member`. |
| `load_indirect` | none | Pops an address handle and pushes its contained value. |
| `store_indirect` | none | Pops value and address handle, stores value to address, and leaves value on stack. |
| `unary` | `string` (op symbol) | Applies unary operation (`!`, `~`, `+`, `-`). |
| `to_bool` | none | Pops value and pushes boolean truth value `0` or `1`. |
| `binary` | `string` (op symbol) | Pops right and left operands, applies binary operation (`+`, `-`, `*`, `/`, `%`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `&`, `\|`, `^`, `<<`, `>>`). |
| `dup` | none | Duplicates top value on stack. |
| `rotate` | none | Rotates top 3 stack values (`a, b, c` -> `b, a, c`); preserves old value in postfix updates. |
| `pop` | none | Discards top value from stack. |
| `call` | `int` (arg count) | Pops argument values and target function, executes call, pushes result. |
| `jump` | `int` (instruction index) | Unconditional jump to target instruction index. |
| `jump_if_false` | `int` (instruction index) | Pops condition; jumps if false (`0`). |
| `jump_if_true` | `int` (instruction index) | Pops condition; jumps if true (non-zero). |
| `return` | none | Returns current stack top value (or `nil` if stack is empty). |
| `make_array` | `int` (array length) | Pushes a new `[]any` slice of specified initial length onto the stack. |
| `make_struct` | none | Pushes a new `map[string]any` map onto the stack. |

### Binary Format Specification (`Instruction.MarshalBinary`)

Instructions can be serialized into a portable, architecture-independent binary stream (`Version 1` layout):

1. **Header (3 bytes):**
   - Byte 0: Format Version (`1`)
   - Byte 1: Opcode numeric identifier (1-21)
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

---

## Detailed C99 Compliance Matrix

Scintilla aims for high alignment with ISO/IEC 9899:1999 (C99) syntax and semantics while maintaining a lightweight runtime execution model.

### 1. Preprocessor Compliance

| C99 Preprocessor Feature | Scintilla Status | Details & Notes |
| --- | --- | --- |
| `#define` (Object-like) | **Supported** | Full macro definition and expansion. |
| `#define` (Function-like) | **Supported** | Parameterized macro expansion with argument substitution. |
| Variadic Macros (`...`, `__VA_ARGS__`) | **Supported** | Variadic arguments in function-like macros. |
| Stringification (`#`) & Token Pasting (`##`) | **Supported** | Converts tokens to string literals (`#`) and pastes tokens (`##`). |
| `#undef` | **Supported** | Removes macro definition. |
| `#include` | **Supported** | Resolver-backed custom header inclusion via `IncludeResolver`. |
| Conditionals (`#if`, `#ifdef`, `#ifndef`, `#elif`, `#else`, `#endif`) | **Supported** | Full conditional compilation evaluation. |
| `defined` Operator | **Supported** | Evaluated during `#if`/`#elif` expansion (`defined(X)` or `defined X`). |
| `#line`, `#error`, `#pragma` | *Not Implemented* | Directives are not yet recognized by the macro expander. |
| Predefined Macros (`__LINE__`, `__FILE__`, etc.) | *Not Implemented* | Standard predefined macros are not automatically injected. |

### 2. Lexical & Language Syntax Compliance

| C99 Syntax Feature | Scintilla Status | Details & Notes |
| --- | --- | --- |
| Keywords | **Supported** | C99 keywords recognized, except for removed keywords (`register`, `volatile`, `restrict`) (`auto`, `break`, `case`, `char`, `const`, `continue`, `default`, `do`, `double`, `else`, `enum`, `extern`, `float`, `for`, `goto`, `if`, `inline`, `int`, `long`, `return`, `short`, `signed`, `sizeof`, `static`, `struct`, `switch`, `typedef`, `union`, `unsigned`, `void`, `while`, `_Bool`, `_Complex`, `_Imaginary`). |
| Comments | **Supported** | Line comments (`//`) and block comments (`/* ... */`). |
| Numeric Literals | **Supported** | Decimal, Hexadecimal (`0x`), Octal (`0`), Floating-point scientific notation (`1e-10`), suffixes (`u`, `l`, `f`). |
| Character & String Literals | **Supported** | Escaped sequences handled by lexer/interpreter. |
| Trigraphs / Digraphs | *Not Implemented* | Alternative token representations are not supported. |

### 3. Statements & Control Flow

| C99 Statement Feature | Scintilla Status | Details & Notes |
| --- | --- | --- |
| Selection (`if`, `if`-`else`) | **Supported** | Translated to conditional jumps. |
| `switch`, `case`, `default` | **Supported** | Full switch statement lowerings with fall-through support and case label validation. |
| Iteration (`while`, `do`-`while`) | **Supported** | Translated to jump constructs. |
| `for` Loops | **Supported** | Includes support for C99 loop-header variable declarations (`for (int i = 0; ...)`). |
| Jump Statements (`break`, `continue`) | **Supported** | Validated during semantic pass for enclosing loop/switch scopes. |
| `goto` & Labeled Statements | **Supported** | Resolved to instruction jump targets in function context. |
| `return` | **Supported** | Supports void and value returns. |

### 4. Declarations & Types

| C99 Type / Declaration Feature | Scintilla Status | Details & Notes |
| --- | --- | --- |
| Primitive Types (`int`, `float`, `char`, `string`, `double`, `void`, etc.) | **Supported** | Lexed, parsed, and evaluated at runtime using Go dynamic representations (`int64`, `float64`, `string`). Includes native `string` type. |
| Structs (`struct`) & Unions (`union`) | **Supported** | Member declaration parsing and runtime `map[string]any` field member access (`.` and `->`). |
| Enumerations (`enum`) | **Supported** | Enumerator constants registered in value symbol scope. |
| Typedefs (`typedef`) | **Supported** | Custom type identifiers tracked in parser and semantic scopes. |
| Array Declarations | **Supported** | Fixed and variable array declarator suffixes parsed; runtime array indexing supported. |
| Specifiers (`const`, `inline`, `auto`, `extern`, `static`) | **Partially Supported** | Parsed in type specifiers/declarators. Storage duration semantics (`static` persistence) and qualifiers are not enforced by VM execution. Note: `register`, `volatile`, and `restrict` keywords have been removed. |
| Standard Library (`<stdio.h>`, `<stdlib.h>`, etc.) | *Divergent* | Standard C library headers are omitted. Native host functions are exposed via Go bindings (`RegisterFunction`). |

### 5. Expressions & Operators

| C99 Expression Feature | Scintilla Status | Details & Notes |
| --- | --- | --- |
| Primary Expressions & Identifiers | **Supported** | Identifiers, constants, string literals, parenthesized expressions. |
| Postfix / Prefix (`++`, `--`) | **Supported** | Address-based update semantics (`Rotate` opcode preserves postfix value). |
| Unary Operators (`+`, `-`, `!`, `~`) | **Supported** | Integer and floating-point unary evaluation. |
| Address-Of (`&`) & Dereference (`*`) | **Supported** | Syntax and lvalue address resolution supported; raw physical memory addresses are omitted. |
| Binary Arithmetic & Bitwise Operators | **Supported** | `+`, `-`, `*`, `/`, `%`, `&`, `\|`, `^`, `<<`, `>>`. |
| Relational & Equality Operators | **Supported** | `<`, `>`, `<=`, `>=`, `==`, `!=`. |
| Logical Operators (`&&`, `\|\|`) | **Supported** | Short-circuit evaluation retained via conditional jumps. |
| Conditional Operator (`?:`) | **Supported** | Short-circuit ternary evaluation retained via conditional jumps. |
| Assignment & Compound Assignment | **Supported** | `=`, `+=`, `-=`, `*=`, `/=`, `%=`, `&=`, `\|=`, `^=`, `<<=`, `>>=`. |
| Comma Expression (`,`) | **Supported** | Sequential evaluation yielding last expression result. |
| Function Calls | **Supported** | Argument stack lowering for guest and host function invocations. |
| Cast Expressions (`(type)expr`) | **Parsed Only** | Syntactically parsed in AST; bytecode translator does not enforce dynamic cast conversions. |
| `sizeof` Operator | **Supported** | Evaluates string length for string operands or byte size for type specifiers / expressions. |
| Compound Literals & Initializer Lists | **Supported** | Full parsing, semantic analysis, bytecode lowering, and VM execution for struct/array compound literals and designated initializer lists. |
| `_Static_assert` | **Parsed Only** | Syntactically parsed in AST; semantic analyzer evaluates condition without compile-time termination. |
