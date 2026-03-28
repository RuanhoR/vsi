package main

import (
	"fmt"
	"os"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/repl"
	run "github.com/RuanhoR/vsi/internal/runner/main"
	"github.com/RuanhoR/vsi/pkg/config"
	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "vsi",
	Short: "VSI Language",
	Long:  "vsi is a fast, need compile language",
}
var RunCommand = &cobra.Command{
	Use:   "run",
	Short: "Run vsi file",
	Run:   RunCompile,
}

func RunCompile(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Not Point .vsi File")
		return
	}
	code, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Println("Error reading file:", err.Error())
		return
	}
	result := parser.ParseString(string(code))
	run.RunNode(*result)
}

var ReplCommand = &cobra.Command{
	Use:   "repl",
	Short: "Run vsi repl",
	Run: func(cmd *cobra.Command, args []string) {
		repl.RunRepl()
	},
}
var VersionCommand = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("vsi version " + config.Version)
	},
}

func main() {
	root.AddCommand(RunCommand)
	root.AddCommand(VersionCommand)
	root.Execute()
}
