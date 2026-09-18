# Scintilla

## Overview

Scintilla is an interpreter that is inspired by the programming language C, specifically, standard C99.
It is developed in Golang (its module is `lang/` at the root of folder). The differences are:

- Lack of manual memory management, memory addresses and inline assembly code inserts.

- The runtime is built by the guest code. Guest code's runtime relies on virtual machine's
  built-in functions.

- Source files are not used. The implementation is directly built into header files to simplify interpretation of Scintilla code.

Despite these features, Scintilla still shares the similar concepts with C.

Scintilla files use the `.sc` extension.

Before interpreting Scintilla code, it is translated to bytecode for performance.

## Example program

**main.sc**:

```c

#include "def.sc"

void Start() {
	CalculationResult Result;
	Result.A = 2;
	Result.B = 2;
	int Result = Result.A + Result.B;
}
```

**def.sc**:

```c


typedef struct CalculationResult {
	int A;
	int B;
} CalculationResult;
```

## Goals

- Make language compliant with the C99 standard, including support of macro definitions, directives (#include, #ifndef etc.),
  structs/typedefs, dynamic/fixed arrays, static functions, while still respecting the differences of Scintilla.

- Allow to integrate external symbols (functions, variables, etc.) in external Golang code.

- Make the Scintilla's lexer, parser, bytecode translator and bytecode interpreter public, modular, extendable and reusable
  in external Golang code.

- Provide debugger to track the interpreter's state, frame etc.

## Current C99 compliance

Scintilla is **not yet C99-compliant**.  C99 compliance is a goal, rather than
a claim about the current implementation.  The lexer recognizes C99 keywords
and operators, the parser currently handles declarations, function definitions,
blocks, expressions, `if`, `switch`, `while`, `do`/`while`, `for`, labels and
the `break`, `continue`, `return`, and `goto` statements.  The semantic pass
checks identifier scopes, forward function calls, assignable assignment targets,
and valid loop/switch control flow.

The macro-expander supports object-like and function-like macros and the
`#define`, `#undef`, conditional, and resolver-backed `#include` directives,
but this is not yet the full C99 preprocessor.  The parser/type system does not
yet translate all C99 syntax: casts, `sizeof`, compound literals, initializer
designators, `_Static_assert`, full aggregate/array layout, type conversions,
and most C99 constraints still need implementation.  The bytecode translator
therefore preserves the semantics of the AST it accepts; it must not be read as
claiming support for unimplemented C99 features.

## Bytecode

`lang/bytecode` translates a semantically valid public parser AST to a typed
stack-machine program.  `bytecode.Translate` first runs semantic analysis, so
translation fails rather than emitting bytecode for unresolved names or invalid
control-flow statements.  `Program.Globals` initializes file-scope variables
and `Program.Functions` stores bodies in source order.  Literal operands are
kept in their original source spelling; numeric parsing and C type conversion
belong to the interpreter.

Every instruction carries its source position.  Jump operands are zero-based
instruction indexes in the containing function or globals sequence.  Unless
noted otherwise, an instruction takes operands from and pushes results onto the
value stack.

### Binary instruction view

`Instruction.MarshalBinary` (and its `Bytes` convenience alias) produces a
stable, architecture-independent binary view suitable for bytecode files or
transport to an interpreter.  Its layout is a one-byte format version (`1`), a
one-byte opcode, a one-byte operand kind and operand payload, followed by the
source position. Strings are UTF-8 bytes prefixed by a big-endian `uint32`; all
integer operands and position numbers are big-endian signed 64-bit values. The
three operand kinds are `0` (no operand), `1` (string), and `2` (integer).
Unsupported opcodes or operand types return an error instead of producing an
ambiguous encoding.

| Instruction | Operand | Effect |
| --- | --- | --- |
| `declare` | variable name | Creates a variable in the current execution scope. |
| `push_literal` | source literal | Pushes an integer, floating, character, or string literal. |
| `load` / `address` | variable name | Pushes a variable's value / address. |
| `address_index` | none | Replaces base and index with an address for `base[index]`. |
| `address_member` | member name | Replaces an aggregate address with an address for its member. |
| `load_indirect` / `store_indirect` | none | Loads through an address / stores a value through an address. `store_indirect` leaves the stored value on the stack. |
| `unary` / `binary` | operator spelling | Applies a C operator represented by the current AST. |
| `to_bool` | none | Replaces a value with C truth value `0` or `1`. |
| `dup` / `pop` | none | Duplicates / discards the stack top. |
| `rotate` | none | Rotates the top three values from `a, b, c` to `b, a, c`; used to retain the old value of a postfix update. |
| `call` | argument count | Calls a function designator followed by that many already-evaluated arguments. |
| `jump` | instruction index | Unconditional transfer. |
| `jump_if_false` / `jump_if_true` | instruction index | Pops a condition and transfers when it is false / true. |
| `return` | none | Returns the optional value currently on the stack; a bare return has no value. |

Assignments, compound assignments, prefix/postfix increment and decrement use
the address instructions so that their C expression result is preserved.
`&&`, `||`, and `?:` are emitted with conditional jumps and consequently retain
their C left-to-right, short-circuit evaluation.  Loops, `switch`, `break`,
`continue`, `goto`, and labels are lowered to resolved jump indexes.

## Go modules

- `lang/`
  - `token/` - Contains the token type definition.
  - `parser/` - Contains the parser implementation. Translates a sequence of tokens into the AST tree.
  - `lexer/` - Contains the lexer implementation.
  - `interpreter/` - Contains the interpreter implementation. It executes bytecode provided by
    the bytecode translator.
  - `bytecode/` - Contains the bytecode translator implementation. It translates the AST tree
    into bytecode.
    Notice that bytecode is not native machine code but the native language optimized specifically
    for the interpreter.
  - `macro/` - Contains the macro expander. It takes a sequence of tokens and expands macro-function
    calls, expressions etc.
  - `semantic/` - Contains the implementation of the semantic analysis.

## Before the code's interpretation

```
lexer -> macro expander -> parser -> semantic analysis -> bytecode translator -> interpreter
```
