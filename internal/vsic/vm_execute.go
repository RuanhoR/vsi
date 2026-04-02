package vsic

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/RuanhoR/vsi/internal/runner/value"
)

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

	case OpDupAt:
		pos := instr.Operands[0].(int) // 要复制的元素的栈位置（从栈底开始的索引）
		if pos < 0 || pos >= len(vm.stack) {
			panic(fmt.Sprintf("invalid stack position for OpDupAt: %d (stack size: %d)", pos, len(vm.stack)))
		}
		val := vm.stack[pos]
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
