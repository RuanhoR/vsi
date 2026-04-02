package vsic

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/runner/value"
)

// 堆栈操作
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

// 辅助方法
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

// 函数调用
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
		fmt.Fprintf(vm.stderr, "[DEBUG] Calling function %s with args %v, returnIP=%d\n", fn.Name, args, vm.ip+1)
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
			fmt.Fprintf(vm.stderr, "[DEBUG] %s:%d executing %v (stack=%d)\n", fn.Name, vm.ip, instr.Opcode, len(vm.stack))
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
				fmt.Fprintf(vm.stderr, "[DEBUG] Return from %s, back to ip=%d, currentFn=%s\n", fn.Name, vm.ip+1, vm.currentFn.Name)
			}
			return nil
		}

		if err := vm.executeInstruction(instr); err != nil {
			return err
		}
		vm.ip++
	}
	vm.tryStack = vm.tryStack[:tryStackLen]
	vm.ip = returnIP - 1
	vm.callStack = vm.callStack[:len(vm.callStack)-1]
	vm.currentCtx = savedCtx
	vm.currentFn = savedFn

	return nil
}

func (vm *VM) doReturn(val interface{}) error {
	if len(vm.callStack) <= 1 {
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
		if idx := parseIndex(prop); idx >= 0 {
			return o.GetItem(idx)
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
	case map[string]interface{}:
		if val, ok := o[prop]; ok {
			return val
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
	instance := value.CreateObject()
	instance.Proto["__class__"] = className
	for name, val := range cls.Properties {
		instance.Proto[name] = val
	}
	for name, method := range cls.Methods {
		instance.Proto[name] = method
	}

	// TODO: 调用构造函数
	return instance
}

func (vm *VM) doImport(source string) (interface{}, error) {
	var importPath string
	if filepath.IsAbs(source) {
		importPath = source
	} else {
		importPath = filepath.Join(vm.baseDir, source)
	}
	if _, ok := vm.modules[importPath]; ok {
		if obj, ok := vm.globals[importPath]; ok {
			return obj, nil
		}
	}
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
	moduleObj := value.CreateObject()
	vm.LoadModule(importedModule)
	for _, export := range importedModule.Exports {
		name := export.Name
		if export.Alias != "" {
			name = export.Alias
		}
		if fn, ok := vm.functions[export.Name]; ok {
			moduleObj.Proto[name] = fn
		}
	}
	vm.globals[importPath] = moduleObj

	return moduleObj, nil
}

// 错误处理
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

// 算术运算
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

// 工具函数
func isTruthy(val interface{}) bool {
	if val == nil {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return true
}

func toNum(val interface{}) (int, bool) {
	switch v := val.(type) {
	case int:
		return v, true
	case *value.VsiNumber:
		return v.Value, true
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n, true
		}
	}
	return 0, false
}

func toString(val interface{}) (string, bool) {
	switch v := val.(type) {
	case string:
		return v, true
	case int:
		return strconv.Itoa(v), true
	case *value.VsiNumber:
		return strconv.Itoa(v.Value), true
	}
	return "", false
}

func parseIndex(s string) int {
	if idx, err := strconv.Atoi(s); err == nil && idx >= 0 {
		return idx
	}
	return -1
}

// 序列化工具函数
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
