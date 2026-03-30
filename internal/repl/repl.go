package repl

import (
	"os"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/vsic"
	"github.com/RuanhoR/vsi/pkg/config"
)

func runExpression(expr string) {
	node := parser.ParseString(expr)
	if node == nil {
		return
	}
	module, err := vsic.CompileModule(node, "<repl>", false)
	if err != nil {
		os.Stderr.WriteString("Compile error: " + err.Error() + "\n")
		return
	}
	vm := vsic.NewVM()
	vm.SetupGlobals()
	if err := vm.Run(module); err != nil {
		os.Stderr.WriteString("Runtime error: " + err.Error() + "\n")
	}
}
func handleInput(input string) {
	switch input {
	case ".help":
		os.Stdout.WriteString("Command: " + "\n  exit: exit repl\n  .help: show help")
	case ".exit":
		os.Exit(0)
	default:
		runExpression(input)
	}
	os.Stdout.WriteString("\r\n> ")
}
func RunRepl() {
	os.Stdout.WriteString(
		"Vsi repl version: " + config.Version + "\n" + "input .help to help\n",
	)
	startInputListener(handleInput)
}
