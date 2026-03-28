package repl

import (
	"os"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	run "github.com/RuanhoR/vsi/internal/runner/main"
	"github.com/RuanhoR/vsi/pkg/config"
)

func runExpression(expr string) {
	node := parser.ParseString(expr)
	run.RunNode(*node)
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
