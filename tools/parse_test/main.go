package main

import (
	"fmt"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/compiler/tokenizr"
)

func main() {
	code := "var a = 0;\nprocess.stdout.write(a)"
	tokens := tokenizr.GenerateTokenizr(code)
	fmt.Println("Tokens:")
	for i, t := range tokens {
		fmt.Printf("%02d: %s %q\n", i, t.Type, t.Data)
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panic recovered:", r)
		}
	}()

	ast := parser.ParseString(code)
	fmt.Println("AST:", ast)
}
