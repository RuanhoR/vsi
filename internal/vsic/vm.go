package vsic

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/runner/value"
	"github.com/RuanhoR/vsi/internal/types"
)

// VM 虚拟机
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

// NewVM 创建虚拟机
func NewVM() *VM {
	return &VM{
		stack:     make([]interface{}, 0, 1024),
		callStack: make([]CallFrame, 0, 64),
		globals:   make(map[string]interface{}),
		functions: make(map[string]*CompiledFunction),
		classes:   make(map[string]*CompiledClass),
		modules:   make(map[string]*Module),
		tryStack:  make([]TryFrame, 0, 16),
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		debug:     os.Getenv("VSIC_DEBUG") != "",
	}
}

// LoadModule 加载模块
func (vm *VM) LoadModule(module *Module) error {
	vm.modules[module.FilePath] = module

	// 加载函数
	for name, fn := range module.Functions {
		vm.functions[name] = fn
	}

	// 加载类
	for name, cls := range module.Classes {
		vm.classes[name] = cls
	}

	// 加载全局变量
	for name, val := range module.Variables {
		vm.globals[name] = val
	}

	// 合并常量池
	vm.constants = append(vm.constants, module.Constants...)

	return nil
}

// Run 运行模块
func (vm *VM) Run(module *Module) error {
	// 设置基础路径
	if module.FilePath != "" {
		vm.baseDir = filepath.Dir(module.FilePath)
	}

	if err := vm.LoadModule(module); err != nil {
		return err
	}

	// 查找入口函数（__main__ > main > 第一个函数）
	var entryFn *CompiledFunction
	if fn, ok := vm.functions["__main__"]; ok {
		entryFn = fn
	} else if fn, ok := vm.functions["main"]; ok {
		entryFn = fn
	} else {
		for _, fn := range module.Functions {
			entryFn = fn
			break
		}
	}

	if entryFn == nil {
		return nil
	}

	// 初始化调用帧
	vm.currentFn = entryFn
	vm.ip = 0
	frame := CallFrame{
		Function: entryFn,
		IP:       0,
		Locals:   make([]interface{}, entryFn.LocalCount),
	}
	vm.callStack = append(vm.callStack, frame)
	vm.currentCtx = &vm.callStack[len(vm.callStack)-1]

	// 执行
	return vm.execute()
}

// execute 执行字节码
func (vm *VM) execute() error {
	for vm.ip < len(vm.currentFn.Instructions) {
		instr := vm.currentFn.Instructions[vm.ip]

		if vm.debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] %s:%d executing %v (stack=%d)\n", vm.currentFn.Name, vm.ip, instr.Opcode, len(vm.stack))
		}

		if err := vm.executeInstruction(instr); err != nil {
			return err
		}

		vm.ip++
	}

	return nil
}

// executeInstruction 执行单条指令
func (vm *VM) executeInstruction(instr Instruction) error {
	switch instr.Opcode {
	case OpNop:
		// 无操作

	case OpHalt:
		return nil

	case OpJump:
		if ip, ok := instr.Operands[0].(int); ok {
			vm.ip = ip - 1
		}

	case OpJumpIf:
		cond := vm.pop()
		if isTruthy(cond) {
			if ip, ok := instr.Operands[0].(int); ok {
				vm.ip = ip - 1
			}
		}

	case OpJumpIfNot:
		cond := vm.pop()
		if !isTruthy(cond) {
			if ip, ok := instr.Operands[0].(int); ok {
				vm.ip = ip - 1
			}
		}

	case OpCall:
		argCount := 0
		if len(instr.Operands) > 0 {
			argCount = instr.Operands[0].(int)
		}
		return vm.callFunction(argCount)

	case OpReturn:
		val := vm.pop()
		return vm.doReturn(val)

	case OpPush:
		vm.push(vm.getConstant(instr.Operands))

	case OpPop:
		vm.pop()

	case OpDup:
		val := vm.peek()
		vm.push(val)

	case OpSwap:
		if len(vm.stack) >= 2 {
			top := len(vm.stack) - 1
			vm.stack[top], vm.stack[top-1] = vm.stack[top-1], vm.stack[top]
		}

	case OpLoad:
		idx := instr.Operands[0].(int)
		vm.push(vm.currentCtx.Locals[idx])

	case OpStore:
		idx := instr.Operands[0].(int)
		vm.currentCtx.Locals[idx] = vm.pop()

	case OpLoadGlobal:
		name := instr.Operands[0].(string)
		if val, ok := vm.globals[name]; ok {
			vm.push(val)
		} else if fn, ok := vm.functions[name]; ok {
			vm.push(fn)
		} else {
			vm.push(nil)
		}

	case OpStoreGlobal:
		name := instr.Operands[0].(string)
		vm.globals[name] = vm.pop()

	case OpGetProp:
		obj := vm.pop()
		prop := instr.Operands[0].(string)
		vm.push(vm.getProperty(obj, prop))

	case OpSetProp:
		val := vm.pop()
		obj := vm.pop()
		prop := instr.Operands[0].(string)
		vm.setProperty(obj, prop, val)

	case OpNewArray:
		count := instr.Operands[0].(int)
		items := make([]interface{}, count)
		for i := count - 1; i >= 0; i-- {
			items[i] = vm.pop()
		}
		vm.push(value.CreateArray(items))

	case OpAdd:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.doAdd(left, right))

	case OpSub:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.doSub(left, right))

	case OpMul:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.doMul(left, right))

	case OpDiv:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.doDiv(left, right))

	case OpEq:
		right := vm.pop()
		left := vm.pop()
		vm.push(left == right)

	case OpNe:
		right := vm.pop()
		left := vm.pop()
		vm.push(left != right)

	case OpLt:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.compare(left, right) < 0)

	case OpLe:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.compare(left, right) <= 0)

	case OpGt:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.compare(left, right) > 0)

	case OpGe:
		right := vm.pop()
		left := vm.pop()
		vm.push(vm.compare(left, right) >= 0)

	case OpNewClass:
		className := instr.Operands[0].(string)
		argCount := instr.Operands[1].(int)
		vm.push(vm.newInstance(className, argCount))

	case OpImport:
		source := instr.Operands[0].(string)
		alias := instr.Operands[1].(string)
		moduleObj, err := vm.doImport(source)
		if err != nil {
			return err
		}
		vm.globals[alias] = moduleObj

	case OpThrow:
		err := vm.pop()
		return vm.doThrow(err)

	case OpTry:
		catchIP := instr.Operands[0].(int)
		finallyIP := instr.Operands[1].(int)
		vm.tryStack = append(vm.tryStack, TryFrame{
			CatchIP:    catchIP,
			FinallyIP:  finallyIP,
			HasCatch:   true,
			HasFinally: true,
		})

	case OpEndTry:
		if len(vm.tryStack) > 0 {
			vm.tryStack = vm.tryStack[:len(vm.tryStack)-1]
		}

	case OpSpread:
		arr := vm.pop()
		if a, ok := arr.(*value.VsiArray); ok {
			for _, item := range a.Items {
				vm.push(item)
			}
		}
	}

	return nil
}

// 辅助方法

func (vm *VM) push(val interface{}) {
	vm.stack = append(vm.stack, val)
}

func (vm *VM) pop() interface{} {
	if len(vm.stack) == 0 {
		return nil
	}
	val := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return val
}

func (vm *VM) peek() interface{} {
	if len(vm.stack) == 0 {
		return nil
	}
	return vm.stack[len(vm.stack)-1]
}

func (vm *VM) getConstant(operands []interface{}) interface{} {
	if len(operands) == 0 {
		return nil
	}
	switch v := operands[0].(type) {
	case int:
		// 优先使用当前函数的常量池
		if vm.currentFn != nil && vm.currentFn.Constants != nil && v < len(vm.currentFn.Constants) {
			return vm.currentFn.Constants[v]
		}
		// 回退到全局常量池
		if v < len(vm.constants) {
			return vm.constants[v]
		}
	case string:
		return v
	}
	return nil
}

func (vm *VM) findLabel(label string) int {
	// 简化实现
	return 0
}

func (vm *VM) callFunction(argCount int) error {
	callee := vm.pop()

	// 参数已经在栈上
	args := make([]interface{}, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = vm.pop()
	}

	switch fn := callee.(type) {
	case *CompiledFunction:
		return vm.callCompiledFunction(fn, args)
	case *value.VsiFunction:
		// 调用内置函数
		result, err := fn.Call(args)
		if err != nil {
			return err
		}
		vm.push(result)
	default:
		return fmt.Errorf("cannot call %T", callee)
	}

	return nil
}

func (vm *VM) callCompiledFunction(fn *CompiledFunction, args []interface{}) error {
	if vm.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Calling function %s with args %v, returnIP=%d\n", fn.Name, args, vm.ip+1)
	}
	// 保存当前状态
	// returnIP 是当前 IP，但 execute() 会在调用后执行 vm.ip++
	// 所以我们需要保存 ip+1 作为返回位置
	returnIP := vm.ip + 1
	savedFn := vm.currentFn
	savedCtx := vm.currentCtx
	frame := CallFrame{
		Function: fn,
		IP:       0,
		Locals:   make([]interface{}, fn.LocalCount),
		ReturnIP: returnIP,
	}

	// 绑定参数
	// 注意：如果是方法，params[0] 对应 this，实际参数从索引 1 开始
	// 需要检查是否有 "this" 参数
	for i, arg := range args {
		if i < fn.LocalCount {
			frame.Locals[i] = arg
		}
	}

	// 保存调用前的 tryStack 长度
	tryStackLen := len(vm.tryStack)

	vm.callStack = append(vm.callStack, frame)
	vm.currentFn = fn
	vm.currentCtx = &vm.callStack[len(vm.callStack)-1]
	vm.ip = 0

	// 执行函数
	for vm.ip < len(vm.currentFn.Instructions) {
		instr := vm.currentFn.Instructions[vm.ip]

		if vm.debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] %s:%d executing %v (stack=%d)\n", fn.Name, vm.ip, instr.Opcode, len(vm.stack))
		}

		// 检查是否是 return 指令
		if instr.Opcode == OpReturn {
			var val interface{}
			if len(vm.stack) > 0 {
				val = vm.pop()
			}
			// 清理 tryStack 到调用前的状态
			vm.tryStack = vm.tryStack[:tryStackLen]
			vm.ip = returnIP - 1 // -1 因为 execute() 会执行 vm.ip++
			vm.callStack = vm.callStack[:len(vm.callStack)-1]
			vm.currentCtx = savedCtx
			vm.currentFn = savedFn
			vm.push(val)
			if vm.debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Return from %s, back to ip=%d, currentFn=%s\n", fn.Name, vm.ip+1, vm.currentFn.Name)
			}
			return nil
		}

		if err := vm.executeInstruction(instr); err != nil {
			return err
		}
		vm.ip++
	}

	// 函数执行完毕（没有 return），恢复调用者状态
	vm.tryStack = vm.tryStack[:tryStackLen]
	vm.ip = returnIP - 1 // -1 因为 execute() 会执行 vm.ip++
	vm.callStack = vm.callStack[:len(vm.callStack)-1]
	vm.currentCtx = savedCtx
	vm.currentFn = savedFn

	return nil
}

func (vm *VM) doReturn(val interface{}) error {
	if len(vm.callStack) <= 1 {
		// 主函数返回
		vm.push(val)
		return nil
	}

	returnIP := vm.currentCtx.ReturnIP
	vm.callStack = vm.callStack[:len(vm.callStack)-1]
	vm.currentCtx = &vm.callStack[len(vm.callStack)-1]
	vm.currentFn = vm.currentCtx.Function
	vm.ip = returnIP

	vm.push(val)
	return nil
}

func (vm *VM) getProperty(obj interface{}, prop string) interface{} {
	switch o := obj.(type) {
	case *value.VsiObject:
		if val, ok := o.Proto[prop]; ok {
			return val
		}
	case *value.VsiArray:
		if prop == "length" {
			return len(o.Items)
		}
		// 数字索引
		if idx := parseIndex(prop); idx >= 0 && idx < len(o.Items) {
			return o.Items[idx]
		}
	case string:
		if prop == "length" {
			return len(o)
		}
		if idx := parseIndex(prop); idx >= 0 && idx < len(o) {
			return string(o[idx])
		}
	case *value.VsiNumber:
		if prop == "toString" {
			return value.CreateFunction("toString", []string{}, func(args []interface{}) (interface{}, error) {
				return fmt.Sprintf("%d", o.Value), nil
			})
		}
	}
	return nil
}

func (vm *VM) setProperty(obj interface{}, prop string, val interface{}) {
	switch o := obj.(type) {
	case *value.VsiObject:
		o.Proto[prop] = val
	case *value.VsiArray:
		if idx := parseIndex(prop); idx >= 0 && idx < len(o.Items) {
			o.Items[idx] = val
		}
	}
}

func (vm *VM) newInstance(className string, argCount int) interface{} {
	cls, ok := vm.classes[className]
	if !ok {
		return nil
	}

	// 创建实例
	instance := value.CreateObject()
	instance.Proto["__class__"] = className

	// 设置属性
	for name, val := range cls.Properties {
		instance.Proto[name] = val
	}

	// 绑定方法
	for name, method := range cls.Methods {
		instance.Proto[name] = method
	}

	// TODO: 调用构造函数

	return instance
}

func (vm *VM) doImport(source string) (interface{}, error) {
	// 解析导入路径
	var importPath string
	if filepath.IsAbs(source) {
		importPath = source
	} else {
		importPath = filepath.Join(vm.baseDir, source)
	}

	// 检查是否已经加载过
	if _, ok := vm.modules[importPath]; ok {
		// 返回已加载的模块对象
		if obj, ok := vm.globals[importPath]; ok {
			return obj, nil
		}
	}

	// 读取导入的模块文件
	code, err := os.ReadFile(importPath)
	if err != nil {
		return nil, fmt.Errorf("cannot import module %s: %v", source, err)
	}

	// 解析 AST
	ast := parser.ParseString(string(code))

	// 编译模块
	importedModule, err := CompileModule(ast, importPath, false)
	if err != nil {
		return nil, fmt.Errorf("compilation error in %s: %v", source, err)
	}

	// 创建模块对象
	moduleObj := value.CreateObject()

	// 加载模块（注册函数和类）
	vm.LoadModule(importedModule)

	// 将导出的函数添加到模块对象
	for _, export := range importedModule.Exports {
		name := export.Name
		if export.Alias != "" {
			name = export.Alias
		}
		if fn, ok := vm.functions[export.Name]; ok {
			moduleObj.Proto[name] = fn
		}
	}

	// 将模块对象存储到全局变量
	vm.globals[importPath] = moduleObj

	return moduleObj, nil
}

func (vm *VM) doThrow(err interface{}) error {
	// 查找最近的 try-catch
	for i := len(vm.tryStack) - 1; i >= 0; i-- {
		frame := vm.tryStack[i]
		if frame.HasCatch {
			vm.ip = frame.CatchIP - 1
			vm.push(err)
			return nil
		}
	}

	return fmt.Errorf("unhandled error: %v", err)
}

func (vm *VM) doAdd(left, right interface{}) interface{} {
	// 数字加法
	ln, lok := toNum(left)
	rn, rok := toNum(right)
	if lok && rok {
		return ln + rn
	}

	// 字符串连接
	ls, lsok := toString(left)
	rs, rsok := toString(right)
	if lsok || rsok {
		return ls + rs
	}

	return nil
}

func (vm *VM) doSub(left, right interface{}) interface{} {
	ln, lok := toNum(left)
	rn, rok := toNum(right)
	if lok && rok {
		return ln - rn
	}
	return nil
}

func (vm *VM) doMul(left, right interface{}) interface{} {
	ln, lok := toNum(left)
	rn, rok := toNum(right)
	if lok && rok {
		return ln * rn
	}
	return nil
}

func (vm *VM) doDiv(left, right interface{}) interface{} {
	ln, lok := toNum(left)
	rn, rok := toNum(right)
	if lok && rok && rn != 0 {
		return ln / rn
	}
	return nil
}

func (vm *VM) compare(left, right interface{}) int {
	ln, lok := toNum(left)
	rn, rok := toNum(right)
	if lok && rok {
		if ln < rn {
			return -1
		} else if ln > rn {
			return 1
		}
		return 0
	}
	return 0
}

// BinarySerialization 二进制序列化
type BinarySerialization struct{}

// Serialize 序列化模块到二进制
func (s *BinarySerialization) Serialize(module *Module) ([]byte, error) {
	buf := make([]byte, 0, 4096)

	// 魔数
	buf = append(buf, 'V', 'S', 'I', 'C')

	// 版本
	buf = appendUint16(buf, 1)

	// 模块名
	buf = appendString(buf, module.Name)

	// 常量池
	buf = appendUint32(buf, uint32(len(module.Constants)))
	for _, c := range module.Constants {
		buf = appendConstant(buf, c)
	}

	// 函数
	buf = appendUint32(buf, uint32(len(module.Functions)))
	for name, fn := range module.Functions {
		buf = appendFunction(buf, name, fn)
	}

	// 类
	buf = appendUint32(buf, uint32(len(module.Classes)))
	for name, cls := range module.Classes {
		buf = appendClass(buf, name, cls)
	}

	return buf, nil
}

// Deserialize 从二进制反序列化模块
func (s *BinarySerialization) Deserialize(data []byte) (*Module, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("invalid vsic file: too small")
	}

	// 检查魔数
	if data[0] != 'V' || data[1] != 'S' || data[2] != 'I' || data[3] != 'C' {
		return nil, fmt.Errorf("invalid vsic magic number")
	}

	offset := 4
	module := &Module{
		Functions: make(map[string]*CompiledFunction),
		Classes:   make(map[string]*CompiledClass),
	}

	// 版本
	if offset+2 > len(data) {
		return nil, fmt.Errorf("invalid vsic file: unexpected end at version")
	}
	_ = binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	// 模块名
	var name string
	name, offset, ok := readStringSafe(data, offset)
	if !ok {
		return nil, fmt.Errorf("invalid vsic file: unexpected end at module name")
	}
	module.Name = name

	// 常量池
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid vsic file: unexpected end at constant count")
	}
	constCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	module.Constants = make([]interface{}, constCount)
	for i := uint32(0); i < constCount; i++ {
		var c interface{}
		c, offset, ok = readConstantSafe(data, offset)
		if !ok {
			return nil, fmt.Errorf("invalid vsic file: unexpected end at constant %d", i)
		}
		module.Constants[i] = c
	}

	// 函数
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid vsic file: unexpected end at function count")
	}
	fnCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	for i := uint32(0); i < fnCount; i++ {
		var name string
		var fn *CompiledFunction
		name, fn, offset, ok = readFunctionSafe(data, offset)
		if !ok {
			return nil, fmt.Errorf("invalid vsic file: unexpected end at function %d", i)
		}
		module.Functions[name] = fn
	}

	// 类
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid vsic file: unexpected end at class count")
	}
	clsCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	for i := uint32(0); i < clsCount; i++ {
		var name string
		var cls *CompiledClass
		name, cls, offset, ok = readClassSafe(data, offset)
		if !ok {
			return nil, fmt.Errorf("invalid vsic file: unexpected end at class %d", i)
		}
		module.Classes[name] = cls
	}

	return module, nil
}

// 辅助序列化函数

func appendUint16(buf []byte, v uint16) []byte {
	buf = append(buf, 0, 0)
	binary.LittleEndian.PutUint16(buf[len(buf)-2:], v)
	return buf
}

func appendUint32(buf []byte, v uint32) []byte {
	buf = append(buf, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(buf[len(buf)-4:], v)
	return buf
}

func appendString(buf []byte, s string) []byte {
	buf = appendUint32(buf, uint32(len(s)))
	buf = append(buf, s...)
	return buf
}

func appendConstant(buf []byte, c interface{}) []byte {
	switch v := c.(type) {
	case nil:
		buf = append(buf, byte(ConstNil))
	case int:
		buf = append(buf, byte(ConstInt))
		buf = appendUint32(buf, uint32(v))
	case string:
		buf = append(buf, byte(ConstString))
		buf = appendString(buf, v)
	case bool:
		buf = append(buf, byte(ConstBool))
		if v {
			buf = append(buf, 1)
		} else {
			buf = append(buf, 0)
		}
	}
	return buf
}

func appendFunction(buf []byte, name string, fn *CompiledFunction) []byte {
	buf = appendString(buf, name)
	buf = appendString(buf, fn.Name)
	buf = appendUint32(buf, uint32(len(fn.Params)))
	for _, p := range fn.Params {
		buf = appendString(buf, p)
	}
	buf = appendUint32(buf, uint32(fn.LocalCount))
	buf = appendUint32(buf, uint32(len(fn.Instructions)))
	for _, instr := range fn.Instructions {
		buf = appendInstruction(buf, instr)
	}
	return buf
}

func appendInstruction(buf []byte, instr Instruction) []byte {
	buf = append(buf, byte(instr.Opcode))
	buf = appendUint32(buf, uint32(len(instr.Operands)))
	for _, op := range instr.Operands {
		switch v := op.(type) {
		case nil:
			buf = append(buf, 0x00) // nil 类型
		case int:
			buf = append(buf, 0x01)
			buf = appendUint32(buf, uint32(v))
		case string:
			buf = append(buf, 0x02)
			buf = appendString(buf, v)
		}
	}
	return buf
}

func appendClass(buf []byte, name string, cls *CompiledClass) []byte {
	buf = appendString(buf, name)
	buf = appendString(buf, cls.Name)
	buf = appendString(buf, cls.Parent)
	// 简化：省略方法和属性的详细序列化
	return buf
}

// 辅助反序列化函数（带边界检查）

func readString(data []byte, offset int) (string, int) {
	length := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	s := string(data[offset : offset+int(length)])
	return s, offset + int(length)
}

// 带边界检查的读取函数
func readStringSafe(data []byte, offset int) (string, int, bool) {
	if offset+4 > len(data) {
		return "", offset, false
	}
	length := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(length) > len(data) {
		return "", offset, false
	}
	s := string(data[offset : offset+int(length)])
	return s, offset + int(length), true
}

func readConstant(data []byte, offset int) (interface{}, int) {
	typ := ConstantType(data[offset])
	offset++
	switch typ {
	case ConstNil:
		return nil, offset
	case ConstInt:
		v := binary.LittleEndian.Uint32(data[offset:])
		return int(v), offset + 4
	case ConstString:
		return readString(data, offset)
	case ConstBool:
		return data[offset] != 0, offset + 1
	}
	return nil, offset
}

func readConstantSafe(data []byte, offset int) (interface{}, int, bool) {
	if offset >= len(data) {
		return nil, offset, false
	}
	typ := ConstantType(data[offset])
	offset++
	switch typ {
	case ConstNil:
		return nil, offset, true
	case ConstInt:
		if offset+4 > len(data) {
			return nil, offset, false
		}
		v := binary.LittleEndian.Uint32(data[offset:])
		return int(v), offset + 4, true
	case ConstString:
		return readStringSafe(data, offset)
	case ConstBool:
		if offset >= len(data) {
			return nil, offset, false
		}
		return data[offset] != 0, offset + 1, true
	}
	return nil, offset, true
}

func readFunction(data []byte, offset int) (string, *CompiledFunction, int) {
	name, offset := readString(data, offset)
	fnName, offset := readString(data, offset)

	paramCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	params := make([]string, paramCount)
	for i := uint32(0); i < paramCount; i++ {
		params[i], offset = readString(data, offset)
	}

	localCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	instrCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	instructions := make([]Instruction, instrCount)
	for i := uint32(0); i < instrCount; i++ {
		instructions[i], offset = readInstruction(data, offset)
	}

	fn := &CompiledFunction{
		Name:         fnName,
		Params:       params,
		LocalCount:   int(localCount),
		Instructions: instructions,
	}

	return name, fn, offset
}

func readFunctionSafe(data []byte, offset int) (string, *CompiledFunction, int, bool) {
	var ok bool
	var name, fnName string
	name, offset, ok = readStringSafe(data, offset)
	if !ok {
		return "", nil, offset, false
	}
	fnName, offset, ok = readStringSafe(data, offset)
	if !ok {
		return "", nil, offset, false
	}

	if offset+4 > len(data) {
		return "", nil, offset, false
	}
	paramCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	params := make([]string, paramCount)
	for i := uint32(0); i < paramCount; i++ {
		params[i], offset, ok = readStringSafe(data, offset)
		if !ok {
			return "", nil, offset, false
		}
	}

	if offset+4 > len(data) {
		return "", nil, offset, false
	}
	localCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	if offset+4 > len(data) {
		return "", nil, offset, false
	}
	instrCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	instructions := make([]Instruction, instrCount)
	for i := uint32(0); i < instrCount; i++ {
		instructions[i], offset, ok = readInstructionSafe(data, offset)
		if !ok {
			return "", nil, offset, false
		}
	}

	fn := &CompiledFunction{
		Name:         fnName,
		Params:       params,
		LocalCount:   int(localCount),
		Instructions: instructions,
	}

	return name, fn, offset, true
}

func readInstruction(data []byte, offset int) (Instruction, int) {
	opcode := Opcode(data[offset])
	offset++

	operandCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	operands := make([]interface{}, operandCount)
	for i := uint32(0); i < operandCount; i++ {
		typ := data[offset]
		offset++
		switch typ {
		case 0x00:
			operands[i] = nil
		case 0x01:
			v := binary.LittleEndian.Uint32(data[offset:])
			operands[i] = int(v)
			offset += 4
		case 0x02:
			operands[i], offset = readString(data, offset)
		}
	}

	return Instruction{Opcode: opcode, Operands: operands}, offset
}

func readInstructionSafe(data []byte, offset int) (Instruction, int, bool) {
	if offset >= len(data) {
		return Instruction{}, offset, false
	}
	opcode := Opcode(data[offset])
	offset++

	if offset+4 > len(data) {
		return Instruction{}, offset, false
	}
	operandCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	operands := make([]interface{}, operandCount)
	for i := uint32(0); i < operandCount; i++ {
		if offset >= len(data) {
			return Instruction{}, offset, false
		}
		typ := data[offset]
		offset++
		switch typ {
		case 0x00:
			operands[i] = nil
		case 0x01:
			if offset+4 > len(data) {
				return Instruction{}, offset, false
			}
			v := binary.LittleEndian.Uint32(data[offset:])
			operands[i] = int(v)
			offset += 4
		case 0x02:
			var s string
			var ok bool
			s, offset, ok = readStringSafe(data, offset)
			if !ok {
				return Instruction{}, offset, false
			}
			operands[i] = s
		}
	}

	return Instruction{Opcode: opcode, Operands: operands}, offset, true
}

func readClass(data []byte, offset int) (string, *CompiledClass, int) {
	name, offset := readString(data, offset)
	clsName, offset := readString(data, offset)
	parent, offset := readString(data, offset)

	cls := &CompiledClass{
		Name:       clsName,
		Parent:     parent,
		Methods:    make(map[string]*CompiledFunction),
		Properties: make(map[string]interface{}),
	}

	return name, cls, offset
}

func readClassSafe(data []byte, offset int) (string, *CompiledClass, int, bool) {
	var ok bool
	var name, clsName, parent string
	name, offset, ok = readStringSafe(data, offset)
	if !ok {
		return "", nil, offset, false
	}
	clsName, offset, ok = readStringSafe(data, offset)
	if !ok {
		return "", nil, offset, false
	}
	parent, offset, ok = readStringSafe(data, offset)
	if !ok {
		return "", nil, offset, false
	}

	cls := &CompiledClass{
		Name:       clsName,
		Parent:     parent,
		Methods:    make(map[string]*CompiledFunction),
		Properties: make(map[string]interface{}),
	}

	return name, cls, offset, true
}

// 辅助函数

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case string:
		return len(val) > 0
	}
	return true
}

func toNum(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case value.VsiNumber:
		return n.Value, true
	case *value.VsiNumber:
		return n.Value, true
	}
	return 0, false
}

func toString(v interface{}) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case value.VsiString:
		return s.Value, true
	case *value.VsiString:
		return s.Value, true
	}
	return fmt.Sprintf("%v", v), true
}

func parseIndex(s string) int {
	var idx int
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			idx = idx*10 + int(ch-'0')
		} else {
			return -1
		}
	}
	return idx
}

// SaveToFile 保存模块到文件
func SaveToFile(module *Module, path string) error {
	serializer := &BinarySerialization{}
	data, err := serializer.Serialize(module)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadFromFile 从文件加载模块
func LoadFromFile(path string) (*Module, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	serializer := &BinarySerialization{}
	return serializer.Deserialize(data)
}

// CompileAndSave 编译并保存
func CompileAndSave(node *types.ProgramNode, sourcePath, outputPath string, optimize bool) error {
	module, err := CompileModule(node, sourcePath, optimize)
	if err != nil {
		return err
	}
	return SaveToFile(module, outputPath)
}

// LoadAndRun 加载并运行
func LoadAndRun(path string) error {
	module, err := LoadFromFile(path)
	if err != nil {
		return err
	}

	vm := NewVM()
	vm.SetupGlobals()
	return vm.Run(module)
}

// SetupGlobals 设置全局对象
func (vm *VM) SetupGlobals() {
	// 添加 process 对象
	process := value.CreateObject()

	// process.cwd - 当前工作目录
	cwd, _ := os.Getwd()
	process.Proto["cwd"] = value.VsiString{Value: cwd}

	// stdout
	stdout := value.CreateObject()
	stdout.Proto["write"] = value.CreateFunction("write", []string{"data"}, func(args []interface{}) (interface{}, error) {
		if len(args) > 0 {
			switch v := args[0].(type) {
			case string:
				fmt.Fprint(vm.stdout, v)
			case int:
				fmt.Fprint(vm.stdout, v)
			case value.VsiString:
				fmt.Fprint(vm.stdout, v.Value)
			case *value.VsiString:
				fmt.Fprint(vm.stdout, v.Value)
			case value.VsiNumber:
				fmt.Fprint(vm.stdout, v.Value)
			case *value.VsiNumber:
				fmt.Fprint(vm.stdout, v.Value)
			case *value.VsiArray:
				// 打印数组元素
				fmt.Fprint(vm.stdout, "[")
				for i, item := range v.Items {
					if i > 0 {
						fmt.Fprint(vm.stdout, " ")
					}
					switch n := item.(type) {
					case int:
						fmt.Fprint(vm.stdout, n)
					case value.VsiNumber:
						fmt.Fprint(vm.stdout, n.Value)
					case *value.VsiNumber:
						fmt.Fprint(vm.stdout, n.Value)
					default:
						fmt.Fprint(vm.stdout, n)
					}
				}
				fmt.Fprint(vm.stdout, "]")
			case *value.VsiObject:
				fmt.Fprint(vm.stdout, "{object}")
			default:
				fmt.Fprint(vm.stdout, v)
			}
		}
		return nil, nil
	})
	process.Proto["stdout"] = stdout

	// file 对象
	fileObj := value.CreateObject()
	fileObj.Proto["readFile"] = value.CreateFunction("readFile", []string{"path"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("readFile requires at least one argument")
		}
		var filePath string
		switch v := args[0].(type) {
		case string:
			filePath = v
		case value.VsiString:
			filePath = v.Value
		case *value.VsiString:
			filePath = v.Value
		default:
			filePath = fmt.Sprint(v)
		}
		result, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		// 返回 int[] (unicode 数组)
		resultArr := make([]interface{}, len(result))
		for i, b := range result {
			resultArr[i] = int(b)
		}
		return value.CreateArray(resultArr), nil
	})
	fileObj.Proto["writeFile"] = value.CreateFunction("writeFile", []string{"path", "data"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("writeFile requires path and data arguments")
		}
		var filePath string
		switch v := args[0].(type) {
		case string:
			filePath = v
		case value.VsiString:
			filePath = v.Value
		case *value.VsiString:
			filePath = v.Value
		default:
			filePath = fmt.Sprint(v)
		}
		var data []byte
		switch v := args[1].(type) {
		case string:
			data = []byte(v)
		case value.VsiString:
			data = []byte(v.Value)
		case *value.VsiString:
			data = []byte(v.Value)
		case *value.VsiArray:
			data = make([]byte, len(v.Items))
			for i, item := range v.Items {
				switch n := item.(type) {
				case int:
					data[i] = byte(n)
				case value.VsiNumber:
					data[i] = byte(n.Value)
				case *value.VsiNumber:
					data[i] = byte(n.Value)
				}
			}
		default:
			data = []byte(fmt.Sprint(v))
		}
		return nil, os.WriteFile(filePath, data, 0644)
	})
	process.Proto["file"] = fileObj

	// path 对象
	pathObj := value.CreateObject()
	pathObj.Proto["join"] = value.CreateFunction("join", []string{"...paths"}, func(args []interface{}) (interface{}, error) {
		parts := []string{}
		for _, a := range args {
			switch v := a.(type) {
			case string:
				parts = append(parts, v)
			case value.VsiString:
				parts = append(parts, v.Value)
			case *value.VsiString:
				parts = append(parts, v.Value)
			default:
				parts = append(parts, fmt.Sprint(v))
			}
		}
		return value.VsiString{Value: filepath.Join(parts...)}, nil
	})
	pathObj.Proto["dirname"] = value.CreateFunction("dirname", []string{"p"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return value.VsiString{Value: ""}, nil
		}
		var p string
		switch v := args[0].(type) {
		case string:
			p = v
		case value.VsiString:
			p = v.Value
		case *value.VsiString:
			p = v.Value
		default:
			p = fmt.Sprint(v)
		}
		return value.VsiString{Value: filepath.Dir(p)}, nil
	})
	pathObj.Proto["basename"] = value.CreateFunction("basename", []string{"p"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return value.VsiString{Value: ""}, nil
		}
		var p string
		switch v := args[0].(type) {
		case string:
			p = v
		case value.VsiString:
			p = v.Value
		case *value.VsiString:
			p = v.Value
		default:
			p = fmt.Sprint(v)
		}
		return value.VsiString{Value: filepath.Base(p)}, nil
	})
	process.Proto["path"] = pathObj

	vm.globals["process"] = process

	// 添加 String 对象
	stringObj := value.CreateObject()
	// String.new(unicodeArray, encoding, length) 或 String.new(unicodeArray)
	// unicodeArray: int[] unicode 编码数组
	// encoding: any 编码方式（可选，目前忽略）
	// length: int 长度（可选）
	stringObj.Proto["new"] = value.CreateFunction("new", []string{"unicodeArray", "encoding", "length"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return value.VsiString{Value: ""}, nil
		}

		// 获取 unicode 数组
		var codes []int
		switch arr := args[0].(type) {
		case *value.VsiArray:
			codes = make([]int, len(arr.Items))
			for i, item := range arr.Items {
				switch v := item.(type) {
				case int:
					codes[i] = v
				case value.VsiNumber:
					codes[i] = v.Value
				case *value.VsiNumber:
					codes[i] = v.Value
				default:
					codes[i] = 0
				}
			}
		default:
			return value.VsiString{Value: ""}, nil
		}

		// 确定长度
		length := len(codes)
		if len(args) >= 3 {
			switch l := args[2].(type) {
			case int:
				if l < length {
					length = l
				}
			case value.VsiNumber:
				if l.Value < length {
					length = l.Value
				}
			case *value.VsiNumber:
				if l.Value < length {
					length = l.Value
				}
			}
		}

		// 转换为字符串
		result := make([]rune, length)
		for i := 0; i < length; i++ {
			result[i] = rune(codes[i])
		}

		return value.VsiString{Value: string(result)}, nil
	})

	// String.fromCharCode
	stringObj.Proto["fromCharCode"] = value.CreateFunction("fromCharCode", []string{"...codes"}, func(args []interface{}) (interface{}, error) {
		result := ""
		for _, arg := range args {
			switch v := arg.(type) {
			case int:
				result += string(rune(v))
			case value.VsiNumber:
				result += string(rune(v.Value))
			case *value.VsiNumber:
				result += string(rune(v.Value))
			case *value.VsiArray:
				for _, item := range v.Items {
					switch n := item.(type) {
					case int:
						result += string(rune(n))
					case value.VsiNumber:
						result += string(rune(n.Value))
					case *value.VsiNumber:
						result += string(rune(n.Value))
					}
				}
			}
		}
		return value.VsiString{Value: result}, nil
	})

	// String.toUnicodeArray - 将字符串转换为 unicode 数组
	stringObj.Proto["toUnicodeArray"] = value.CreateFunction("toUnicodeArray", []string{"str"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return value.CreateArray([]interface{}{}), nil
		}
		var s string
		switch v := args[0].(type) {
		case string:
			s = v
		case value.VsiString:
			s = v.Value
		case *value.VsiString:
			s = v.Value
		default:
			s = fmt.Sprint(v)
		}
		// 转换为 unicode 数组
		items := make([]interface{}, len(s))
		for i, ch := range s {
			items[i] = int(ch)
		}
		return value.CreateArray(items), nil
	})

	vm.globals["String"] = stringObj

	// 添加 Error 对象
	errorObj := value.CreateObject()
	errorObj.Proto["new"] = value.CreateFunction("new", []string{"message"}, func(args []interface{}) (interface{}, error) {
		msg := ""
		if len(args) > 0 {
			switch v := args[0].(type) {
			case string:
				msg = v
			case value.VsiString:
				msg = v.Value
			case *value.VsiString:
				msg = v.Value
			default:
				msg = fmt.Sprint(v)
			}
		}
		err := value.CreateObject()
		err.Proto["Message"] = value.VsiString{Value: msg}
		err.Proto["ErrorType"] = value.VsiString{Value: "Error"}
		return err, nil
	})
	vm.globals["Error"] = errorObj

	// 添加基础类型
	vm.globals["int"] = "int"
	vm.globals["string"] = "string"
	vm.globals["bool"] = "bool"
}
