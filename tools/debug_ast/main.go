package main

import (
	"fmt"
	"os"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
)

func main() {
	code, err := os.ReadFile("example/TestVSIC.vsi")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	ast := parser.ParseString(string(code))
	
	fmt.Println("AST Body:")
	for i, node := range ast.Body {
		fmt.Printf("%d: %T - %v\n", i, node, node)
	}
}
