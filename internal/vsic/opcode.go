package vsic

// Opcode 定义 VSIC 字节码操作码
type Opcode byte

const (
	// 控制流
	OpNop      Opcode = 0x00 // 无操作
	OpHalt     Opcode = 0x01 // 停止执行
	OpJump     Opcode = 0x02 // 无条件跳转
	OpJumpIf   Opcode = 0x03 // 条件跳转（真）
	OpJumpIfNot Opcode = 0x04 // 条件跳转（假）
	OpCall     Opcode = 0x05 // 调用函数
	OpReturn   Opcode = 0x06 // 返回

	// 栈操作
	OpPush     Opcode = 0x10 // 压入常量
	OpPop      Opcode = 0x11 // 弹出栈顶
	OpDup      Opcode = 0x12 // 复制栈顶
	OpSwap     Opcode = 0x13 // 交换栈顶两个元素

	// 变量操作
	OpLoad     Opcode = 0x20 // 加载变量
	OpStore    Opcode = 0x21 // 存储变量
	OpLoadGlobal Opcode = 0x22 // 加载全局变量
	OpStoreGlobal Opcode = 0x23 // 存储全局变量

	// 对象操作
	OpGetProp  Opcode = 0x30 // 获取属性
	OpSetProp  Opcode = 0x31 // 设置属性
	OpNewObj   Opcode = 0x32 // 创建对象
	OpNewArray Opcode = 0x33 // 创建数组

	// 运算符
	OpAdd      Opcode = 0x40 // 加法
	OpSub      Opcode = 0x41 // 减法
	OpMul      Opcode = 0x42 // 乘法
	OpDiv      Opcode = 0x43 // 除法
	OpEq       Opcode = 0x44 // 等于
	OpNe       Opcode = 0x45 // 不等于
	OpLt       Opcode = 0x46 // 小于
	OpLe       Opcode = 0x47 // 小于等于
	OpGt       Opcode = 0x48 // 大于
	OpGe       Opcode = 0x49 // 大于等于
	OpNot      Opcode = 0x4A // 逻辑非
	OpNeg      Opcode = 0x4B // 负号

	// 类和实例
	OpNewClass  Opcode = 0x50 // 创建类实例
	OpGetMethod Opcode = 0x51 // 获取方法

	// 异常处理
	OpThrow    Opcode = 0x60 // 抛出异常
	OpTry      Opcode = 0x61 // 开始 try
	OpCatch    Opcode = 0x62 // catch 块
	OpFinally  Opcode = 0x63 // finally 块
	OpEndTry   Opcode = 0x64 // 结束 try

	// 模块
	OpImport   Opcode = 0x70 // 导入模块
	OpExport   Opcode = 0x71 // 导出

	// 展开运算符
	OpSpread   Opcode = 0x80 // 展开数组
)

// ConstantType 常量类型
type ConstantType byte

const (
	ConstNil    ConstantType = 0x00
	ConstInt    ConstantType = 0x01
	ConstString ConstantType = 0x02
	ConstBool   ConstantType = 0x03
	ConstFloat  ConstantType = 0x04
)

// Instruction 表示一条指令
type Instruction struct {
	Opcode   Opcode
	Operands []interface{}
	Line     int    // 源码行号（用于调试）
	Comment  string // 注释（用于调试）
}

// Function 表示编译后的函数
type CompiledFunction struct {
	Name       string
	Params     []string
	ParamTypes []string
	ReturnType string
	Instructions []Instruction
	LocalCount int // 局部变量数量
}

// Class 表示编译后的类
type CompiledClass struct {
	Name       string
	Parent     string
	Methods    map[string]*CompiledFunction
	Properties map[string]interface{}
	StaticMethods map[string]*CompiledFunction
}

// Module 表示编译后的模块
type Module struct {
	Name       string
	FilePath   string
	Functions  map[string]*CompiledFunction
	Classes    map[string]*CompiledClass
	Variables  map[string]interface{}
	Imports    []ImportInfo
	Exports    []ExportInfo
	Constants  []interface{} // 常量池
	EntryPoint int           // 主入口点
}

// ImportInfo 导入信息
type ImportInfo struct {
	Source string
	Alias  string
}

// ExportInfo 导出信息
type ExportInfo struct {
	Name  string
	Alias string
}

// Program 表示完整的编译后程序
type Program struct {
	MainModule string
	Modules    map[string]*Module
	Constants  []interface{} // 全局常量池
	Metadata   ProgramMetadata
}

// ProgramMetadata 程序元数据
type ProgramMetadata struct {
	Version    string
	CompileTime string
	SourceFiles []string
}
