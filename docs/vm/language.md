# Language

## Overview

The virtual machine's language (called Scintilla) is inspired by the programming language C, specifically, standard C99.
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
  structs/typedefs, dynamic/fixed arrays, static functions.

- Allow to integrate own interpreter symbols in Golang.

- Make the Scintilla's lexer, parser, bytecode translator and bytecode interpreter public, modular and reusable
  in external Golang code.
