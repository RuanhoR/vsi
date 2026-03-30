package main

import (
	"fmt"
	"os"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/vsic"
)

func main() {
	code, _ := os.ReadFile("example/TestIf.vsi")
	ast := parser.ParseString(string(code))

	module, err := vsic.CompileModule(ast, "test", false)
	if err != nil {
		fmt.Println("Compile error:", err)
		return
	}

	fmt.Println("Instructions:")
	if fn, ok := module.Functions["__main__"]; ok {
		for i, instr := range fn.Instructions {
			fmt.Printf("  %3d: Opcode=%d Operands=%v\n", i, instr.Opcode, instr.Operands)
		}
	}

	fmt.Println("\nConstants:")
	for i, c := range module.Constants {
		fmt.Printf("  %d: %v\n", i, c)
	}

	fmt.Println("\nRunning...")
	vm := vsic.NewVM()
	vm.SetupGlobals()
	if err := vm.Run(module); err != nil {
		fmt.Println("Error:", err)
	}
}
