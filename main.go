package main

import (
	"fmt"
	"os"
	"tvshow/lang/bytecode"
	"tvshow/lang/interpreter"
	"tvshow/lang/lexer"
	"tvshow/lang/macro"
	"tvshow/lang/parser"
	"tvshow/lang/semantic"
	"tvshow/lang/token"
)

func getTokens(filepath string) ([]token.Token, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	l := lexer.New(filepath, string(content))
	return l.Tokens(), nil
}

func expandMacros(tokens []token.Token) error {
	expander := macro.New()
	expander.Include = func(name string, pos token.Position) ([]token.Token, error) {
		tokens, err := getTokens(name)
		if err != nil {
			return nil, fmt.Errorf("\"#include %q\": failed to read file", name)
		}
		return tokens, nil
	}
	err := error(nil)
	tokens, err = expander.Expand(tokens)
	if err != nil {
		return nil
	}
	return nil
}

func parse(tokens []token.Token) (*parser.Program, error) {
	p := parser.New(tokens)
	program, err := p.ParseProgram()
	if err != nil {
		return nil, err
	}
	return program, nil
}

func doSemantic(p *parser.Program) error {
	return semantic.Analyze(p)
}

func doBytecode(p *parser.Program) (*bytecode.Program, error) {
	return bytecode.Translate(p)
}

func main() {
	filepath := "main.sc"

	tokens, err := getTokens(filepath)
	if err != nil {
		panic(err)
	}

	err = expandMacros(tokens)
	if err != nil {
		panic(err)
	}

	program, err := parse(tokens)
	if err != nil {
		panic(err)
	}

	err = doSemantic(program)
	if err != nil {
		panic(err)
	}

	bp, err := doBytecode(program)
	if err != nil {
		panic(err)
	}

	i := interpreter.New(bp)
	f, _ := interpreter.WrapFunc(func(v any) {
		fmt.Printf("%v\n", v)
	})
	i.RegisterFunction("__BuiltinPrint", f)

	_, err = i.Run("Start")
	if err != nil {
		panic(err)
	}
}
