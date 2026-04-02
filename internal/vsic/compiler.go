package vsic

import (
	"fmt"

	"github.com/RuanhoR/vsi/internal/runner/value"
	"github.com/RuanhoR/vsi/internal/types"
)

// Compiler 编译器
type Compiler struct {
	constants     []interface{}
	functions     map[string]*CompiledFunction
	classes       map[string]*CompiledClass
	currentFunc   *CompiledFunction
	instructions  []Instruction
	labels        map[string]int
	labelCounter  int
	variables     map[string]int // 变量名 -> 局部变量索引
	globalVars    map[string]bool
	optimizations []OptimizationPass
}

// OptimizationPass 优化_pass 接口
type OptimizationPass interface {
	Apply(*Module) error
	Name() string
}

// NewCompiler 创建编译器
func NewCompiler() *Compiler {
	return &Compiler{
		constants:    make([]interface{}, 0),
		functions:    make(map[string]*CompiledFunction),
		classes:      make(map[string]*CompiledClass),
		labels:       make(map[string]int),
		labelCounter: 0,
		variables:    make(map[string]int),
		globalVars:   make(map[string]bool),
	}
}

// Compile 编译 AST 到模块
func (c *Compiler) Compile(node *types.ProgramNode, filePath string) *Module {
	module := &Module{
		Name:      filePath,
		FilePath:  filePath,
		Functions: make(map[string]*CompiledFunction),
		Classes:   make(map[string]*CompiledClass),
		Variables: make(map[string]interface{}),
		Imports:   make([]ImportInfo, 0),
		Exports:   make([]ExportInfo, 0),
		Constants: make([]interface{}, 0),
	}

	// 创建主函数上下文（用于编译全局代码）
	c.currentFunc = &CompiledFunction{
		Name:       "__main__",
		Params:     []string{},
		LocalCount: 0,
	}
	c.variables = make(map[string]int)
	c.instructions = make([]Instruction, 0)

	// 第一遍：收集所有声明
	for _, n := range node.Body {
		switch stmt := n.(type) {
		case *types.FunctionDeclaration:
			c.collectFunction(stmt)
		case *types.ClassDeclaration:
			c.collectClass(stmt)
		}
	}

	// 第二遍：编译所有声明
	for _, n := range node.Body {
		switch stmt := n.(type) {
		case *types.FunctionDeclaration:
			fn := c.compileFunction(stmt)
			module.Functions[fn.Name] = fn
		case *types.ClassDeclaration:
			cls := c.compileClass(stmt)
			module.Classes[cls.Name] = cls
		case *types.VarDefineDeclaration:
			c.compileStatement(stmt)
		case *types.ConstDefineDeclaration:
			c.compileStatement(stmt)
		case *types.ImportDeclaration:
			// 生成 OpImport 指令
			c.emit(OpImport, stmt.Source, stmt.Alias)
			module.Imports = append(module.Imports, ImportInfo{
				Source: stmt.Source,
				Alias:  stmt.Alias,
			})
		case *types.ExportDeclaration:
			for _, item := range stmt.Items {
				module.Exports = append(module.Exports, ExportInfo{
					Name:  item.Name,
					Alias: item.Alias,
				})
			}
		case *types.ExpressionStatement:
			c.compileStatement(stmt)
		case *types.IfStatement:
			c.compileStatement(stmt)
		case *types.ForStatement:
			c.compileStatement(stmt)
		case *types.WhileStatement:
			c.compileStatement(stmt)
		case *types.ThrowStatement:
			c.compileStatement(stmt)
		case *types.TryStatement:
			c.compileStatement(stmt)
		}
	}

	// 添加 halt 指令到主函数
	c.emit(OpHalt)

	// 解析标签（将标签名替换为实际偏移量）
	c.resolveLabels()

	// 保存主函数
	c.currentFunc.Instructions = c.instructions
	module.Functions["__main__"] = c.currentFunc

	// 设置常量池
	module.Constants = c.constants

	// 应用优化
	for _, opt := range c.optimizations {
		opt.Apply(module)
	}

	return module
}

// collectFunction 收集函数信息（第一遍）
func (c *Compiler) collectFunction(decl *types.FunctionDeclaration) {
	c.functions[decl.Name] = &CompiledFunction{
		Name:       decl.Name,
		Params:     paramNames(decl.Params),
		ParamTypes: paramTypes(decl.Params),
		ReturnType: decl.ReturnType,
	}
}

// collectClass 收集类信息（第一遍）
func (c *Compiler) collectClass(decl *types.ClassDeclaration) {
	c.classes[decl.Name] = &CompiledClass{
		Name:          decl.Name,
		Parent:        decl.Parent,
		Methods:       make(map[string]*CompiledFunction),
		Properties:    make(map[string]interface{}),
		StaticMethods: make(map[string]*CompiledFunction),
	}
}

// compileFunction 编译函数
func (c *Compiler) compileFunction(decl *types.FunctionDeclaration) *CompiledFunction {
	// 保存当前状态
	oldFunc := c.currentFunc
	oldVars := c.variables
	oldInstructions := c.instructions
	oldConstants := c.constants

	// 创建新的函数上下文
	c.currentFunc = &CompiledFunction{
		Name:       decl.Name,
		Params:     paramNames(decl.Params),
		ParamTypes: paramTypes(decl.Params),
		ReturnType: decl.ReturnType,
	}
	c.variables = make(map[string]int)
	c.instructions = make([]Instruction, 0)
	c.constants = make([]interface{}, 0) // 每个函数有自己独立的常量池

	// 设置参数为局部变量
	for i, param := range decl.Params {
		c.variables[param.Name] = i
	}
	c.currentFunc.LocalCount = len(decl.Params)

	// 编译函数体
	for _, stmt := range decl.Body.Body {
		c.compileStatement(stmt)
	}

	// 添加隐式 return
	if len(c.instructions) == 0 || c.instructions[len(c.instructions)-1].Opcode != OpReturn {
		c.emit(OpReturn, nil)
	}

	// 解析标签
	c.resolveLabels()

	// 保存编译好的函数
	compiledFunc := c.currentFunc
	compiledFunc.Instructions = c.instructions
	compiledFunc.Constants = c.constants // 保存函数的常量池

	// 恢复状态
	c.currentFunc = oldFunc
	c.variables = oldVars
	c.instructions = oldInstructions
	c.constants = oldConstants

	return compiledFunc
}

// compileClass 编译类
func (c *Compiler) compileClass(decl *types.ClassDeclaration) *CompiledClass {
	cls := &CompiledClass{
		Name:          decl.Name,
		Parent:        decl.Parent,
		Methods:       make(map[string]*CompiledFunction),
		Properties:    make(map[string]interface{}),
		StaticMethods: make(map[string]*CompiledFunction),
	}

	// 编译属性
	for _, prop := range decl.Properties {
		if prop.Value != nil {
			cls.Properties[prop.Name] = c.evalConstant(prop.Value)
		}
	}

	// 编译方法
	for i := range decl.Methods {
		method := &decl.Methods[i]
		fn := c.compileMethod(method, decl.Name)
		if method.IsStatic {
			cls.StaticMethods[method.Name] = fn
		} else {
			cls.Methods[method.Name] = fn
		}
	}

	c.classes[decl.Name] = cls
	return cls
}

// compileMethod 编译方法
func (c *Compiler) compileMethod(method *types.ClassMethod, className string) *CompiledFunction {
	// 保存当前状态
	oldFunc := c.currentFunc
	oldVars := c.variables
	oldInstructions := c.instructions
	oldConstants := c.constants

	// 创建新的函数上下文
	c.currentFunc = &CompiledFunction{
		Name:       method.Name,
		Params:     paramNames(method.Params),
		ParamTypes: paramTypes(method.Params),
		ReturnType: method.ReturnType,
	}
	c.variables = make(map[string]int)
	c.instructions = make([]Instruction, 0)
	c.constants = make([]interface{}, 0) // 每个方法有自己独立的常量池

	// this 是第一个参数
	c.variables["this"] = 0

	// 设置参数为局部变量
	for i, param := range method.Params {
		c.variables[param.Name] = i + 1 // +1 因为 this 在索引 0
	}
	c.currentFunc.LocalCount = len(method.Params) + 1

	// 编译方法体
	for _, stmt := range method.Body.Body {
		c.compileStatement(stmt)
	}

	// 添加隐式 return
	if len(c.instructions) == 0 || c.instructions[len(c.instructions)-1].Opcode != OpReturn {
		c.emit(OpReturn, nil)
	}

	c.currentFunc.Instructions = c.instructions
	c.currentFunc.Constants = c.constants // 保存方法的常量池

	// 保存编译好的方法
	compiledMethod := c.currentFunc

	// 恢复状态
	c.currentFunc = oldFunc
	c.variables = oldVars
	c.instructions = oldInstructions
	c.constants = oldConstants

	return compiledMethod
}

// compileStatement 编译语句
func (c *Compiler) compileStatement(stmt types.Statement) {
	switch s := stmt.(type) {
	case *types.ExpressionStatement:
		c.compileExpression(s.Expression)
		c.emit(OpPop)

	case *types.ReturnStatement:
		if s.Argument != nil {
			c.compileExpression(s.Argument)
		} else {
			c.emit(OpPush, nil)
		}
		c.emit(OpReturn)

	case *types.VarDefineDeclaration:
		for _, d := range s.Declarations {
			if d.Value != nil {
				c.compileExpression(d.Value)
			} else {
				c.emit(OpPush, nil)
			}
			idx := c.addLocal(d.Name)
			c.emit(OpStore, idx)
		}

	case *types.ConstDefineDeclaration:
		for _, d := range s.Declarations {
			c.compileExpression(d.Value)
			idx := c.addLocal(d.Name)
			c.emit(OpStore, idx)
		}

	case *types.IfStatement:
		c.compileIfStatement(s)

	case *types.ForStatement:
		c.compileForStatement(s)

	case *types.WhileStatement:
		c.compileWhileStatement(s)

	case *types.ThrowStatement:
		c.compileExpression(s.Argument)
		c.emit(OpThrow)

	case *types.TryStatement:
		c.compileTryStatement(s)
	}
}

// compileExpression 编译表达式
func (c *Compiler) compileExpression(expr types.Expression) {
	switch e := expr.(type) {
	case *types.NumberLiteral:
		idx := c.addConstant(parseInt(e.Value))
		c.emit(OpPush, idx)

	case *types.StringLiteral:
		idx := c.addConstant(e.Value)
		c.emit(OpPush, idx)

	case *types.Identifier:
		if idx, ok := c.variables[e.Name]; ok {
			c.emit(OpLoad, idx)
		} else {
			// 全局变量或函数
			c.emit(OpLoadGlobal, e.Name)
		}

	case *types.BinaryExpression:
		c.compileBinaryExpression(e)

	case *types.MemberExpression:
		c.compileExpression(e.Object)
		c.emit(OpGetProp, e.Property)

	case *types.CallExpression:
		c.compileCallExpression(e)

	case *types.ArrayExpression:
		for _, elem := range e.Elements {
			c.compileExpression(elem)
		}
		c.emit(OpNewArray, len(e.Elements))

	case *types.NewExpression:
		c.compileNewExpression(e)

	case *types.SpreadExpression:
		c.compileExpression(e.Argument)
		c.emit(OpSpread)

	default:
		c.emit(OpPush, nil)
	}
}

// compileBinaryExpression 编译二元表达式
func (c *Compiler) compileBinaryExpression(expr *types.BinaryExpression) {
	// 赋值特殊处理
	if expr.Operator == "=" {
		// 右侧值编译（结果在栈顶）
		c.compileExpression(expr.Right)
		// 存储到左侧
		c.compileAssignment(expr.Left)
		return
	}

	// 其他运算符
	c.compileExpression(expr.Left)
	c.compileExpression(expr.Right)

	switch expr.Operator {
	case "+":
		c.emit(OpAdd)
	case "-":
		c.emit(OpSub)
	case "*":
		c.emit(OpMul)
	case "/":
		c.emit(OpDiv)
	case "==":
		c.emit(OpEq)
	case "!=":
		c.emit(OpNe)
	case "<":
		c.emit(OpLt)
	case "<=":
		c.emit(OpLe)
	case ">":
		c.emit(OpGt)
	case ">=":
		c.emit(OpGe)
	}
}

// compileAssignment 编译赋值
// 假设右侧值已经在栈顶
func (c *Compiler) compileAssignment(left types.Expression) {
	switch l := left.(type) {
	case *types.Identifier:
		if idx, ok := c.variables[l.Name]; ok {
			c.emit(OpStore, idx)
		} else {
			c.emit(OpStoreGlobal, l.Name)
		}
	case *types.MemberExpression:
		// 需要先计算对象，然后设置属性
		// 栈: [..., value] -> 需要变成 [..., obj, value]
		// 复制值
		c.emit(OpDup)                 // [..., value, value]
		c.compileExpression(l.Object) // [..., value, value, obj]
		// 交换栈顶两个元素
		c.emit(OpSwap) // [..., value, obj, value]
		c.emit(OpSetProp, l.Property)
	}
}

// compileCallExpression 编译调用表达式
func (c *Compiler) compileCallExpression(expr *types.CallExpression) {
	// 检查是否是方法调用 (obj.method())
	// 只有当对象是变量或属性访问时才当作方法调用
	if member, ok := expr.Callee.(*types.MemberExpression); ok {
		// 检查 member.Object 是否是简单的标识符（变量）
		// 如果是，那就是用户对象的方法调用，需要传递 this
		// 如果是链式访问（如 process.stdout.write），当作普通函数调用
		if _, isIdent := member.Object.(*types.Identifier); isIdent {
			// 可能是方法调用，也可能是模块访问
			// 我们假设模块访问不会产生 CompiledFunction，所以方法调用需要 this
			// 方法调用
			// 栈布局目标: [this, arg1, arg2, ..., method] -> OpCall (argCount+1)

			// 1. 先编译 this (对象)，放在栈底
			c.compileExpression(member.Object)

			// 2. 编译参数
			argCount := 0
			for _, arg := range expr.Arguments {
				if spread, ok := arg.(*types.SpreadExpression); ok {
					c.compileExpression(spread.Argument)
					c.emit(OpSpread)
				} else {
					c.compileExpression(arg)
					argCount++
				}
			}

			// 3. 获取方法 (复制 this 对象，然后获取属性)
			c.emit(OpDupAt, 0) // 复制栈底的 this 对象
			c.emit(OpGetProp, member.Property)

			// 4. 调用 (参数数量 + 1 for this)
			c.emit(OpCall, argCount+1)
			return
		}
	}

	// 普通函数调用
	argCount := 0
	for _, arg := range expr.Arguments {
		if spread, ok := arg.(*types.SpreadExpression); ok {
			c.compileExpression(spread.Argument)
			c.emit(OpSpread)
		} else {
			c.compileExpression(arg)
			argCount++
		}
	}

	// 编译被调用者
	c.compileExpression(expr.Callee)

	// 发出调用指令
	c.emit(OpCall, argCount)
}

// compileNewExpression 编译 new 表达式
func (c *Compiler) compileNewExpression(expr *types.NewExpression) {
	// 编译参数
	for _, arg := range expr.Arguments {
		c.compileExpression(arg)
	}

	// 获取类名
	var className string
	if id, ok := expr.Class.(*types.Identifier); ok {
		className = id.Name
	}

	c.emit(OpNewClass, className, len(expr.Arguments))
}

// compileIfStatement 编译 if 语句
func (c *Compiler) compileIfStatement(stmt *types.IfStatement) {
	// 条件
	c.compileExpression(stmt.Test)

	// 条件跳转
	elseLabel := c.newLabel()
	endLabel := c.newLabel()
	c.emit(OpJumpIfNot, elseLabel)

	// then 分支
	for _, s := range stmt.Consequent.Body {
		c.compileStatement(s)
	}
	c.emit(OpJump, endLabel)

	// else 分支
	c.setLabel(elseLabel)
	if stmt.Alternate != nil {
		switch alt := stmt.Alternate.(type) {
		case *types.BlockStatement:
			for _, s := range alt.Body {
				c.compileStatement(s)
			}
		case *types.IfStatement:
			c.compileIfStatement(alt)
		}
	}

	c.setLabel(endLabel)
}

// compileForStatement 编译 for 语句
func (c *Compiler) compileForStatement(stmt *types.ForStatement) {
	// 初始化
	if stmt.Init != nil {
		c.compileStatement(stmt.Init)
	}

	startLabel := c.newLabel()
	endLabel := c.newLabel()

	c.setLabel(startLabel)

	// 条件
	if stmt.Test != nil {
		c.compileExpression(stmt.Test)
		c.emit(OpJumpIfNot, endLabel)
	}

	// 循环体
	for _, s := range stmt.Body.Body {
		c.compileStatement(s)
	}

	// 更新
	if stmt.Update != nil {
		c.compileExpression(stmt.Update)
		c.emit(OpPop)
	}

	c.emit(OpJump, startLabel)
	c.setLabel(endLabel)
}

// compileWhileStatement 编译 while 语句
func (c *Compiler) compileWhileStatement(stmt *types.WhileStatement) {
	startLabel := c.newLabel()
	endLabel := c.newLabel()

	c.setLabel(startLabel)

	// 条件
	c.compileExpression(stmt.Test)
	c.emit(OpJumpIfNot, endLabel)

	// 循环体
	for _, s := range stmt.Body.Body {
		c.compileStatement(s)
	}

	c.emit(OpJump, startLabel)
	c.setLabel(endLabel)
}

// compileTryStatement 编译 try 语句
func (c *Compiler) compileTryStatement(stmt *types.TryStatement) {
	catchLabel := c.newLabel()
	finallyLabel := c.newLabel()
	endLabel := c.newLabel()

	c.emit(OpTry, catchLabel, finallyLabel)

	// try 块
	for _, s := range stmt.Block.Body {
		c.compileStatement(s)
	}
	// try 块正常结束，跳转到 finally
	c.emit(OpJump, finallyLabel)

	// catch 块
	c.setLabel(catchLabel)
	if stmt.Catch != nil {
		// 错误变量
		idx := c.addLocal(stmt.Catch.Param)
		c.emit(OpStore, idx)

		for _, s := range stmt.Catch.Body.Body {
			c.compileStatement(s)
		}
	}
	// catch 块结束，跳转到 finally
	c.emit(OpJump, finallyLabel)

	// finally 块 - 正常执行或 catch 后都会到达这里
	c.setLabel(finallyLabel)
	if stmt.Finally != nil {
		for _, s := range stmt.Finally.Body {
			c.compileStatement(s)
		}
	}

	c.setLabel(endLabel)
	c.emit(OpEndTry)
}

func (c *Compiler) compileVarDeclaration(decl *types.VarDefineDeclaration, module *Module) {
	for _, d := range decl.Declarations {
		if d.Value != nil {
			c.compileExpression(d.Value)
		} else {
			c.emit(OpPush, nil)
		}
		c.emit(OpStoreGlobal, d.Name)
	}
}

// compileConstDeclaration 编译 const 声明
func (c *Compiler) compileConstDeclaration(decl *types.ConstDefineDeclaration, module *Module) {
	for _, d := range decl.Declarations {
		c.compileExpression(d.Value)
		c.emit(OpStoreGlobal, d.Name)
	}
}

// compileExpressionStatement 编译表达式语句
func (c *Compiler) compileExpressionStatement(stmt *types.ExpressionStatement, module *Module) {
	c.compileExpression(stmt.Expression)
}

// 辅助方法

func (c *Compiler) emit(opcode Opcode, operands ...interface{}) {
	instr := Instruction{
		Opcode:   opcode,
		Operands: operands,
	}
	c.instructions = append(c.instructions, instr)
}

func (c *Compiler) addConstant(value interface{}) int {
	c.constants = append(c.constants, value)
	return len(c.constants) - 1
}

func (c *Compiler) addLocal(name string) int {
	// 如果变量已存在，返回已有索引
	if idx, exists := c.variables[name]; exists {
		return idx
	}
	idx := len(c.variables)
	c.variables[name] = idx
	if c.currentFunc != nil && idx >= c.currentFunc.LocalCount {
		c.currentFunc.LocalCount = idx + 1
	}
	return idx
}

func (c *Compiler) newLabel() string {
	label := fmt.Sprintf("L%d", c.labelCounter)
	c.labelCounter++
	return label
}

func (c *Compiler) setLabel(name string) {
	c.labels[name] = len(c.instructions)
}

// resolveLabels 解析标签，将标签名替换为实际指令偏移量
func (c *Compiler) resolveLabels() {
	for i := range c.instructions {
		instr := &c.instructions[i]
		// 处理跳转指令
		if instr.Opcode == OpJump || instr.Opcode == OpJumpIf || instr.Opcode == OpJumpIfNot {
			if len(instr.Operands) > 0 {
				if labelName, ok := instr.Operands[0].(string); ok {
					if targetIP, exists := c.labels[labelName]; exists {
						instr.Operands[0] = targetIP
					}
				}
			}
		}
		// 处理 try 指令
		if instr.Opcode == OpTry {
			if len(instr.Operands) >= 2 {
				if catchLabel, ok := instr.Operands[0].(string); ok {
					if targetIP, exists := c.labels[catchLabel]; exists {
						instr.Operands[0] = targetIP
					}
				}
				if finallyLabel, ok := instr.Operands[1].(string); ok {
					if targetIP, exists := c.labels[finallyLabel]; exists {
						instr.Operands[1] = targetIP
					}
				}
			}
		}
	}
	// 清空标签映射以便下次使用
	c.labels = make(map[string]int)
}

func (c *Compiler) evalConstant(expr types.Expression) interface{} {
	switch e := expr.(type) {
	case *types.NumberLiteral:
		return parseInt(e.Value)
	case *types.StringLiteral:
		return e.Value
	}
	return nil
}

func paramNames(params []types.Parameter) []string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}
	return names
}

func paramTypes(params []types.Parameter) []string {
	types := make([]string, len(params))
	for i, p := range params {
		types[i] = p.Type
	}
	return types
}

func parseInt(s string) *value.VsiNumber {
	var n int
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return &value.VsiNumber{Value: n}
}

// AddOptimization 添加优化 pass
func (c *Compiler) AddOptimization(opt OptimizationPass) {
	c.optimizations = append(c.optimizations, opt)
}
