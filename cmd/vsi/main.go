package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/repl"
	"github.com/RuanhoR/vsi/internal/vsic"
	"github.com/RuanhoR/vsi/pkg/config"
	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "vsi",
	Short: "VSI Language",
	Long:  "vsi is a fast, need compile language",
}

var runOptimize bool
var RunCommand = &cobra.Command{
	Use:   "run <file.vsi>",
	Short: "Run vsi file",
	Long:  "Compile and run a VSI source file (uses VSIC bytecode internally)",
	Args:  cobra.MinimumNArgs(1),
	Run:   RunCompile,
}

func RunCompile(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Not Point .vsi File")
		return
	}

	start := time.Now()
	sourcePath := args[0]

	code, err := os.ReadFile(sourcePath)
	if err != nil {
		fmt.Println("Error reading file:", err.Error())
		return
	}

	// 解析 AST
	ast := parser.ParseString(string(code))

	// 编译到 VSIC 字节码（内存中）
	module, err := vsic.CompileModule(ast, sourcePath, runOptimize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		os.Exit(1)
	}

	// 运行 VSIC
	vm := vsic.NewVM()
	vm.SetupGlobals()

	if err := vm.Run(module); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	_ = elapsed // 不显示时间，除非用户请求
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

// BuildCommand 编译命令
var buildOutput string
var buildOptimize bool
var BuildCommand = &cobra.Command{
	Use:   "build <source.vsi>",
	Short: "Compile VSI to VSIC bytecode",
	Long:  "Compile a VSI source file to optimized VSIC bytecode",
	Args:  cobra.MinimumNArgs(1),
	Run:   RunBuild,
}

func RunBuild(cmd *cobra.Command, args []string) {
	sourcePath := args[0]

	// 确定输出路径
	outputPath := buildOutput
	if outputPath == "" {
		ext := filepath.Ext(sourcePath)
		outputPath = sourcePath[:len(sourcePath)-len(ext)] + ".vsic"
	}

	start := time.Now()

	// 读取源文件
	code, err := os.ReadFile(sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// 解析 AST
	ast := parser.ParseString(string(code))

	// 编译到字节码
	err = vsic.CompileAndSave(ast, sourcePath, outputPath, buildOptimize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Printf("Compiled %s -> %s (%.2fms)\n", sourcePath, outputPath, float64(elapsed.Microseconds())/1000)
}

// VSICCommand 运行 VSIC 字节码
var VSICCommand = &cobra.Command{
	Use:   "vsic <file.vsic>",
	Short: "Run VSIC bytecode",
	Long:  "Run a compiled VSIC bytecode file",
	Args:  cobra.MinimumNArgs(1),
	Run:   RunVSIC,
}

func RunVSIC(cmd *cobra.Command, args []string) {
	path := args[0]
	err := vsic.LoadAndRun(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// run 命令参数
	RunCommand.Flags().BoolVarP(&runOptimize, "optimize", "O", false, "Enable optimizations (experimental)")
	// build 命令参数
	BuildCommand.Flags().StringVarP(&buildOutput, "output", "o", "", "Output file path")
	BuildCommand.Flags().BoolVarP(&buildOptimize, "optimize", "O", false, "Enable optimizations (experimental)")
}

func main() {
	root.AddCommand(RunCommand)
	root.AddCommand(VersionCommand)
	root.AddCommand(BuildCommand)
	root.AddCommand(VSICCommand)
	root.Execute()
}
