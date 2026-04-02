package vsic

import "io"

// VM 虚拟机结构定义
type VM struct {
	stack     []interface{}
	callStack []CallFrame
	globals   map[string]interface{}
	functions map[string]*CompiledFunction
	classes   map[string]*CompiledClass
	modules   map[string]*Module
	constants []interface{}

	// 当前执行状态
	ip         int // 指令指针
	currentFn  *CompiledFunction
	currentCtx *CallFrame

	// 错误处理
	tryStack []TryFrame

	// 输出
	stdout io.Writer
	stderr io.Writer

	// 调试
	debug bool

	// 基础路径（用于解析相对导入）
	baseDir string
}

// CallFrame 调用帧
type CallFrame struct {
	Function    *CompiledFunction
	IP          int
	Locals      []interface{}
	ReturnIP    int
	ReturnValue interface{}
}

// TryFrame try 块帧
type TryFrame struct {
	CatchIP    int
	FinallyIP  int
	HasCatch   bool
	HasFinally bool
}