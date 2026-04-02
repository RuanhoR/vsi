package vsic

import (
	"encoding/gob"
	"fmt"
	"os"
	"time"

	"github.com/RuanhoR/vsi/internal/types"
)

type Opcode byte

const (
	OpNop         Opcode = 0x00 // 无操作
	OpHalt        Opcode = 0x01 // 停止执行
	OpJump        Opcode = 0x02 // 无条件跳转
	OpJumpIf      Opcode = 0x03 // 条件跳转（真）
	OpJumpIfNot   Opcode = 0x04 // 条件跳转（假）
	OpCall        Opcode = 0x05 // 调用函数
	OpReturn      Opcode = 0x06 // 返回
	OpPush        Opcode = 0x10 // 压入常量
	OpPop         Opcode = 0x11 // 弹出栈顶
	OpDup         Opcode = 0x12 // 复制栈顶
	OpDupAt       Opcode = 0x14 // 复制栈中指定位置的元素
	OpSwap        Opcode = 0x13 // 交换栈顶两个元素
	OpLoad        Opcode = 0x20 // 加载变量
	OpStore       Opcode = 0x21 // 存储变量
	OpLoadGlobal  Opcode = 0x22 // 加载全局变量
	OpStoreGlobal Opcode = 0x23 // 存储全局变量
	OpGetProp     Opcode = 0x30 // 获取属性
	OpSetProp     Opcode = 0x31 // 设置属性
	OpNewObj      Opcode = 0x32 // {
	OpNewArray    Opcode = 0x33 // [
	OpAdd         Opcode = 0x40 // +
	OpSub         Opcode = 0x41 // -
	OpMul         Opcode = 0x42 // *
	OpDiv         Opcode = 0x43 // /
	OpEq          Opcode = 0x44 // =
	OpNe          Opcode = 0x45 // !=
	OpLt          Opcode = 0x46 // <
	OpLe          Opcode = 0x47 // <=
	OpGt          Opcode = 0x48 // >
	OpGe          Opcode = 0x49 // >=
	OpNot         Opcode = 0x4A // !
	OpNeg         Opcode = 0x4B // -
	OpNewClass    Opcode = 0x50
	OpGetMethod   Opcode = 0x51
	OpThrow       Opcode = 0x60
	OpTry         Opcode = 0x61 //  try
	OpCatch       Opcode = 0x62 // catch
	OpFinally     Opcode = 0x63 // finally
	OpEndTry      Opcode = 0x64
	OpImport      Opcode = 0x70
	OpExport      Opcode = 0x71
	OpSpread      Opcode = 0x80
)

type ConstantType byte

const (
	ConstNil    ConstantType = 0x00
	ConstInt    ConstantType = 0x01
	ConstString ConstantType = 0x02
	ConstBool   ConstantType = 0x03
	ConstFloat  ConstantType = 0x04
)

type Instruction struct {
	Opcode   Opcode
	Operands []interface{}
	Line     int
	Comment  string
}
type CompiledFunction struct {
	Name         string
	Params       []string
	ParamTypes   []string
	ReturnType   string
	Instructions []Instruction
	LocalCount   int
	Constants    []interface{}
}

type CompiledClass struct {
	Name          string
	Parent        string
	Methods       map[string]*CompiledFunction
	Properties    map[string]interface{}
	StaticMethods map[string]*CompiledFunction
}

type Module struct {
	Name       string
	FilePath   string
	Functions  map[string]*CompiledFunction
	Classes    map[string]*CompiledClass
	Variables  map[string]interface{}
	Imports    []ImportInfo
	Exports    []ExportInfo
	Constants  []interface{}
	EntryPoint int
}

type ImportInfo struct {
	Source string
	Alias  string
}

type ExportInfo struct {
	Name  string
	Alias string
}

type Program struct {
	MainModule string
	Modules    map[string]*Module
	Constants  []interface{}
	Metadata   ProgramMetadata
}

type ProgramMetadata struct {
	Version     string
	CompileTime string
	SourceFiles []string
}

func CompileAndSave(ast *types.ProgramNode, sourcePath, outputPath string, optimize bool) error {
	// 编译模块
	module, err := CompileModule(ast, sourcePath, optimize)
	if err != nil {
		return fmt.Errorf("compilation error: %w", err)
	}

	// 创建程序结构
	program := &Program{
		MainModule: sourcePath,
		Modules: map[string]*Module{
			sourcePath: module,
		},
		Constants: module.Constants,
		Metadata: ProgramMetadata{
			Version:     "1.0.0",
			CompileTime: time.Now().Format(time.RFC3339),
			SourceFiles: []string{sourcePath},
		},
	}

	// 将字节码保存到文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// 使用gob编码保存
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(program)
	if err != nil {
		return fmt.Errorf("failed to encode bytecode: %w", err)
	}

	return nil
}

// LoadAndRun 加载字节码文件并运行
func LoadAndRun(bytecodePath string) error {
	// 读取字节码文件
	file, err := os.Open(bytecodePath)
	if err != nil {
		return fmt.Errorf("failed to open bytecode file: %w", err)
	}
	defer file.Close()

	// 使用gob解码
	decoder := gob.NewDecoder(file)
	var program Program
	err = decoder.Decode(&program)
	if err != nil {
		return fmt.Errorf("failed to decode bytecode: %w", err)
	}

	// 加载主模块并运行
	if mainModule, exists := program.Modules[program.MainModule]; exists {
		vm := NewVM()
		vm.SetupGlobals()
		return vm.Run(mainModule)
	}

	return fmt.Errorf("main module not found in bytecode")
}
