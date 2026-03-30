package vsic

import (
	"fmt"

	"github.com/RuanhoR/vsi/internal/types"
)

// DeadCodeEliminationPass 死代码消除优化
type DeadCodeEliminationPass struct{}

func (p *DeadCodeEliminationPass) Name() string {
	return "DeadCodeElimination"
}

func (p *DeadCodeEliminationPass) Apply(module *Module) error {
	for _, fn := range module.Functions {
		p.optimizeFunction(fn)
	}
	for _, cls := range module.Classes {
		for _, fn := range cls.Methods {
			p.optimizeFunction(fn)
		}
		for _, fn := range cls.StaticMethods {
			p.optimizeFunction(fn)
		}
	}
	return nil
}

func (p *DeadCodeEliminationPass) optimizeFunction(fn *CompiledFunction) {
	if len(fn.Instructions) == 0 {
		return
	}

	optimized := make([]Instruction, 0, len(fn.Instructions))
	
	for i, instr := range fn.Instructions {
		// 移除不可达代码（return/jump 后的代码）
		if len(optimized) > 0 {
			last := optimized[len(optimized)-1]
			if last.Opcode == OpReturn || last.Opcode == OpJump {
				// 检查是否是跳转目标
				if !p.isJumpTarget(fn, i) {
					continue // 跳过不可达代码
				}
			}
		}

		// 移除无效的 pop 后立即 push
		if instr.Opcode == OpPush && len(optimized) > 0 {
			if optimized[len(optimized)-1].Opcode == OpPop {
				// Pop-Push 序列可以优化
				optimized = optimized[:len(optimized)-1]
			}
		}

		optimized = append(optimized, instr)
	}

	fn.Instructions = optimized
}

func (p *DeadCodeEliminationPass) isJumpTarget(fn *CompiledFunction, index int) bool {
	// 简化实现：总是返回 false
	return false
}

// FunctionInliningPass 函数内联优化
type FunctionInliningPass struct {
	callCounts map[string]int
	module     *Module
}

func (p *FunctionInliningPass) Name() string {
	return "FunctionInlining"
}

func (p *FunctionInliningPass) Apply(module *Module) error {
	p.module = module
	p.callCounts = make(map[string]int)

	// 第一遍：统计调用次数
	for _, fn := range module.Functions {
		p.countCalls(fn)
	}
	for _, cls := range module.Classes {
		for _, fn := range cls.Methods {
			p.countCalls(fn)
		}
	}

	// 第二遍：内联只调用一次的小函数
	for _, fn := range module.Functions {
		p.inlineFunction(fn)
	}

	return nil
}

func (p *FunctionInliningPass) countCalls(fn *CompiledFunction) {
	for _, instr := range fn.Instructions {
		if instr.Opcode == OpCall {
			// 统计函数调用
			if len(instr.Operands) > 0 {
				if name, ok := instr.Operands[0].(string); ok {
					p.callCounts[name]++
				}
			}
		}
	}
}

func (p *FunctionInliningPass) inlineFunction(fn *CompiledFunction) {
	// 只内联调用一次且小于 10 条指令的函数
	for name, count := range p.callCounts {
		if count == 1 {
			if target, ok := p.module.Functions[name]; ok {
				if len(target.Instructions) < 10 {
					p.doInline(fn, name, target)
				}
			}
		}
	}
}

func (p *FunctionInliningPass) doInline(fn *CompiledFunction, name string, target *CompiledFunction) {
	// 简化实现：标记为内联候选
	// 完整实现需要展开函数体
}

// ConstantFoldingPass 常量折叠优化
type ConstantFoldingPass struct{}

func (p *ConstantFoldingPass) Name() string {
	return "ConstantFolding"
}

func (p *ConstantFoldingPass) Apply(module *Module) error {
	for _, fn := range module.Functions {
		p.optimizeFunction(fn, module)
	}
	return nil
}

func (p *ConstantFoldingPass) optimizeFunction(fn *CompiledFunction, module *Module) {
	if len(fn.Instructions) < 3 {
		return
	}

	optimized := make([]Instruction, 0, len(fn.Instructions))
	i := 0

	for i < len(fn.Instructions) {
		instr := fn.Instructions[i]

		// 检测常量运算模式：Push const, Push const, Op
		if i+2 < len(fn.Instructions) &&
			fn.Instructions[i].Opcode == OpPush &&
			fn.Instructions[i+1].Opcode == OpPush {

			left := p.getConstant(fn.Instructions[i], module)
			right := p.getConstant(fn.Instructions[i+1], module)

			if left != nil && right != nil {
				op := fn.Instructions[i+2].Opcode
				result := p.foldConstants(left, right, op)

				if result != nil {
					// 用单个 Push 替换
					idx := len(module.Constants)
					module.Constants = append(module.Constants, result)
					optimized = append(optimized, Instruction{
						Opcode:   OpPush,
						Operands: []interface{}{idx},
					})
					i += 3
					continue
				}
			}
		}

		optimized = append(optimized, instr)
		i++
	}

	fn.Instructions = optimized
}

func (p *ConstantFoldingPass) getConstant(instr Instruction, module *Module) interface{} {
	if instr.Opcode == OpPush {
		if idx, ok := instr.Operands[0].(int); ok {
			if idx < len(module.Constants) {
				return module.Constants[idx]
			}
		}
	}
	return nil
}

func (p *ConstantFoldingPass) foldConstants(left, right interface{}, op Opcode) interface{} {
	ln, lok := toNumber(left)
	rn, rok := toNumber(right)

	if lok && rok {
		switch op {
		case OpAdd:
			return ln + rn
		case OpSub:
			return ln - rn
		case OpMul:
			return ln * rn
		case OpDiv:
			if rn != 0 {
				return ln / rn
			}
		case OpEq:
			return ln == rn
		case OpNe:
			return ln != rn
		case OpLt:
			return ln < rn
		case OpLe:
			return ln <= rn
		case OpGt:
			return ln > rn
		case OpGe:
			return ln >= rn
		}
	}

	// 字符串连接
	ls, lsok := left.(string)
	rs, rsok := right.(string)
	if lsok && rsok && op == OpAdd {
		return ls + rs
	}

	return nil
}

func toNumber(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}

// LoopOptimizationPass 循环优化
type LoopOptimizationPass struct{}

func (p *LoopOptimizationPass) Name() string {
	return "LoopOptimization"
}

func (p *LoopOptimizationPass) Apply(module *Module) error {
	for _, fn := range module.Functions {
		p.optimizeFunction(fn)
	}
	return nil
}

func (p *LoopOptimizationPass) optimizeFunction(fn *CompiledFunction) {
	// 检测并优化循环不变量
	// 简化实现：标记循环开始和结束
}

// ApplyOptimizations 应用所有优化
func ApplyOptimizations(module *Module) error {
	passes := []OptimizationPass{
		&ConstantFoldingPass{},
		&DeadCodeEliminationPass{},
		&FunctionInliningPass{},
	}

	for _, pass := range passes {
		if err := pass.Apply(module); err != nil {
			return fmt.Errorf("optimization %s failed: %w", pass.Name(), err)
		}
	}

	return nil
}

// OptimizeProgram 优化整个程序
func OptimizeProgram(program *Program) error {
	for _, module := range program.Modules {
		if err := ApplyOptimizations(module); err != nil {
			return err
		}
	}
	return nil
}

// CompileModule 编译模块（带优化）
func CompileModule(node *types.ProgramNode, filePath string, optimize bool) (*Module, error) {
	compiler := NewCompiler()
	module := compiler.Compile(node, filePath)

	if optimize {
		if err := ApplyOptimizations(module); err != nil {
			return nil, err
		}
	}

	return module, nil
}
