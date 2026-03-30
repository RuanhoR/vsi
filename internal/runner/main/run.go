package run

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/RuanhoR/vsi/internal/compiler/parser"
	"github.com/RuanhoR/vsi/internal/compiler/tokenizr"
	"github.com/RuanhoR/vsi/internal/runner/value"
	"github.com/RuanhoR/vsi/internal/types"
)

// VsiError 运行时错误
type VsiError struct {
	Message string
	Stack   []types.StackFrame
}

func (e *VsiError) Error() string {
	return e.Message
}

// RunNode 执行程序 AST 节点
func RunNode(node types.ProgramNode) map[string]*value.VsiVariable {
	return RunNodeWithFile(node, "")
}

// RunNodeWithFile 执行程序 AST 节点，指定文件名
func RunNodeWithFile(node types.ProgramNode, filename string) map[string]*value.VsiVariable {
	Type := node.Type()
	if Type != "Program" {
		panic("Expected Program Node")
	}
	context := CreateProgramContext(filename)
	exports := make(map[string]*value.VsiVariable)

	defer func() {
		if r := recover(); r != nil {
			handleRuntimeError(context, r)
		}
	}()

	for i := range node.Body {
		execNodeInContext(context, node.Body[i], true, exports)
	}
	return exports
}

// handleRuntimeError 处理运行时错误
func handleRuntimeError(context *ProgramContext, err interface{}) {
	switch e := err.(type) {
	case *types.VsiError:
		e.Stack = append(e.Stack, context.Stack...)
		printError(context, e)
	case *VsiError:
		printVsiError(context, e)
	case error:
		fmt.Printf("Error: %s\n", e.Error())
		if len(context.Stack) > 0 {
			fmt.Println("Stack trace:")
			fmt.Print(context.FormatStack())
		}
	default:
		fmt.Printf("Error: %v\n", err)
		if len(context.Stack) > 0 {
			fmt.Println("Stack trace:")
			fmt.Print(context.FormatStack())
		}
	}
}

// printError 打印 VsiError
func printError(context *ProgramContext, err *types.VsiError) {
	fmt.Printf("%s: %s\n", err.ErrorType, err.Message)
	if len(err.Stack) > 0 {
		fmt.Println("Stack trace:")
		for i := len(err.Stack) - 1; i >= 0; i-- {
			fmt.Printf("  at %s\n", err.Stack[i].String())
		}
	} else if len(context.Stack) > 0 {
		fmt.Println("Stack trace:")
		fmt.Print(context.FormatStack())
	}
}

// printVsiError 打印内部 VsiError
func printVsiError(context *ProgramContext, err *VsiError) {
	fmt.Printf("Error: %s\n", err.Message)
	if len(err.Stack) > 0 {
		fmt.Println("Stack trace:")
		for i := len(err.Stack) - 1; i >= 0; i-- {
			fmt.Printf("  at %s\n", err.Stack[i].String())
		}
	}
}

// execNodeInContext 在上下文中执行节点
func execNodeInContext(context *ProgramContext, node types.BaseNode, isTop bool, exports map[string]*value.VsiVariable) {
	switch node.Type() {
	case "FunctionDeclaration":
		execFunctionDeclaration(context, node.(*types.FunctionDeclaration), isTop)
	case "VarDefineDeclaration":
		execVarDefineDeclaration(context, node.(*types.VarDefineDeclaration), isTop)
	case "ConstDefineDeclaration":
		execConstDefineDeclaration(context, node.(*types.ConstDefineDeclaration), isTop)
	case "ExportDeclaration":
		execExportDeclaration(context, node.(*types.ExportDeclaration), exports)
	case "ExpressionStatement":
		stmt := node.(*types.ExpressionStatement)
		evalExpression(context, stmt.Expression)
	case "ClassDeclaration":
		execClassDeclaration(context, node.(*types.ClassDeclaration), isTop)
	case "ThrowStatement":
		execThrowStatement(context, node.(*types.ThrowStatement))
	case "TryStatement":
		execTryStatement(context, node.(*types.TryStatement), isTop, exports)
	case "IfStatement":
		execIfStatement(context, node.(*types.IfStatement), isTop, exports)
	case "ForStatement":
		execForStatement(context, node.(*types.ForStatement), isTop, exports)
	case "WhileStatement":
		execWhileStatement(context, node.(*types.WhileStatement), isTop, exports)
	case "ReturnStatement":
		// return handled by function execution
		ret := node.(*types.ReturnStatement)
		var result interface{} = nil
		if ret.Argument != nil {
			result = evalExpression(context, ret.Argument)
		}
		panic(&ReturnPanic{Value: result})
	case "ImportDeclaration":
		execImportDeclaration(context, node.(*types.ImportDeclaration), isTop)
	default:
		// other node types not yet implemented
	}
}

// ReturnPanic 用于处理 return 语句
type ReturnPanic struct {
	Value interface{}
}

// execFunctionDeclaration 执行函数声明
func execFunctionDeclaration(context *ProgramContext, decl *types.FunctionDeclaration, isTop bool) {
	fn := value.CreateFunction(decl.Name, funcNames(decl.Params), func(args []interface{}) (interface{}, error) {
		// 压入调用栈
		frame := types.StackFrame{
			File:     context.CurrentFile,
			Function: decl.Name,
			Line:     0, // TODO: 添加行号
			Column:   0,
		}
		context.PushStack(frame)
		defer context.PopStack()

		// 创建局部作用域
		localCtx := context.CreateLocalContext()
		context.EnterContext(localCtx)
		defer context.ExitContext()

		// 绑定参数并进行类型检查
		for i, p := range decl.Params {
			var val interface{} = nil
			if i < len(args) {
				val = args[i]
			}

			// 类型检查
			if p.Type != "" && val != nil {
				if !checkType(val, p.Type, context) {
					return nil, fmt.Errorf("type error: parameter '%s' expects type '%s', got '%s'",
						p.Name, p.Type, getTypeName(val))
				}
			}

			localCtx.Variables[p.Name] = value.CreateVariable(val)
		}

		// 执行函数体
		var result interface{} = nil
		func() {
			defer func() {
				if r := recover(); r != nil {
					if ret, ok := r.(*ReturnPanic); ok {
						result = ret.Value
					} else {
						panic(r)
					}
				}
			}()
			for _, stmt := range decl.Body.Body {
				execNodeInContext(context, stmt, false, nil)
			}
		}()

		// 返回值类型检查
		if decl.ReturnType != "" {
			if !checkType(result, decl.ReturnType, context) {
				return nil, fmt.Errorf("type error: function '%s' expects return type '%s', got '%s'",
					decl.Name, decl.ReturnType, getTypeName(result))
			}
		}

		return result, nil
	})
	context.SetFunction(decl.Name, fn, isTop)
}

// execVarDefineDeclaration 执行 var 声明
func execVarDefineDeclaration(context *ProgramContext, decl *types.VarDefineDeclaration, isTop bool) {
	for _, d := range decl.Declarations {
		varVal := interface{}(nil)
		if d.Value != nil {
			varVal = evalExpression(context, d.Value)
		}
		v := value.CreateVariable(varVal)
		context.SetVariable(d.Name, v, isTop)
	}
}

// execConstDefineDeclaration 执行 const 声明
func execConstDefineDeclaration(context *ProgramContext, decl *types.ConstDefineDeclaration, isTop bool) {
	for _, d := range decl.Declarations {
		varVal := interface{}(nil)
		if d.Value != nil {
			varVal = evalExpression(context, d.Value)
		}
		v := value.CreateVariable(varVal)
		v.Const = true
		context.SetVariable(d.Name, v, isTop)
	}
}

// execExportDeclaration 执行 export 声明
func execExportDeclaration(context *ProgramContext, decl *types.ExportDeclaration, exports map[string]*value.VsiVariable) {
	for _, item := range decl.Items {
		// 先查找变量
		if vv, ok := context.LookupVariable(item.Name); ok {
			exports[item.Name] = vv
		} else if fn, ok := context.LookupFunction(item.Name); ok {
			// 如果是函数，导出函数
			exports[item.Name] = value.CreateVariable(fn)
		} else if cls, ok := context.LookupClass(item.Name); ok {
			// 如果是类，导出类定义
			exports[item.Name] = value.CreateVariable(cls)
		}

		// 处理别名
		if item.Alias != "" {
			if e, ok := exports[item.Name]; ok {
				exports[item.Alias] = e
			}
		}
	}
}

// execImportDeclaration 执行 import 声明
func execImportDeclaration(context *ProgramContext, decl *types.ImportDeclaration, isTop bool) {
	// 解析导入路径
	importPath := decl.Source

	// 如果是相对路径，基于当前文件路径解析
	if !filepath.IsAbs(importPath) {
		baseDir := filepath.Dir(context.CurrentFile)
		importPath = filepath.Join(baseDir, importPath)
	}

	// 读取导入的文件
	content, err := os.ReadFile(importPath)
	if err != nil {
		panic(fmt.Sprintf("import error: cannot read file '%s': %v", decl.Source, err))
	}

	// 解析导入的文件
	tokens := tokenizr.GenerateTokenizr(string(content))
	ast := parser.ParseTokens(tokens)

	// 执行导入的文件，获取导出的内容
	exports := RunNodeWithFile(*ast, importPath)

	// 创建模块对象
	moduleObj := value.CreateObject()

	// 将导出的内容绑定到模块对象
	for name, v := range exports {
		moduleObj.Proto[name] = v.Value
	}

	// 如果有别名，将模块对象绑定到当前上下文
	if decl.Alias != "" {
		context.SetVariable(decl.Alias, value.CreateVariable(moduleObj), isTop)
	}
}

// execClassDeclaration 执行类声明
func execClassDeclaration(context *ProgramContext, decl *types.ClassDeclaration, isTop bool) {
	context.SetClass(decl.Name, decl, isTop)
}

// execThrowStatement 执行 throw 语句
func execThrowStatement(context *ProgramContext, stmt *types.ThrowStatement) {
	err := evalExpression(context, stmt.Argument)

	// 如果是 VsiError，添加当前调用栈
	if vsiErr, ok := err.(*types.VsiError); ok {
		vsiErr.Stack = append(vsiErr.Stack, context.Stack...)
		panic(vsiErr)
	}

	// 否则创建新的错误
	panic(&types.VsiError{
		Message:   fmt.Sprintf("%v", err),
		ErrorType: "Error",
		Stack:     append([]types.StackFrame{}, context.Stack...),
	})
}

// execTryStatement 执行 try 语句
func execTryStatement(context *ProgramContext, stmt *types.TryStatement, isTop bool, exports map[string]*value.VsiVariable) {
	var caughtErr interface{} = nil
	var hasError bool = false

	// 执行 try 块
	func() {
		defer func() {
			if r := recover(); r != nil {
				// 检查是否是 return 语句
				if _, isReturn := r.(*ReturnPanic); isReturn {
					panic(r) // 继续向上抛出 return
				}
				caughtErr = r
				hasError = true
			}
		}()
		for _, s := range stmt.Block.Body {
			execNodeInContext(context, s, isTop, exports)
		}
	}()

	// 执行 catch 块
	if hasError && stmt.Catch != nil {
		// 创建局部作用域
		localCtx := context.CreateLocalContext()
		context.EnterContext(localCtx)

		// 绑定错误变量
		var errValue interface{} = caughtErr
		if vsiErr, ok := caughtErr.(*types.VsiError); ok {
			errValue = vsiErr
		}
		localCtx.Variables[stmt.Catch.Param] = value.CreateVariable(errValue)

		// 执行 catch 块
		func() {
			defer func() {
				if r := recover(); r != nil {
					if _, isReturn := r.(*ReturnPanic); isReturn {
						panic(r)
					}
					caughtErr = r
					hasError = true
				} else {
					hasError = false
				}
			}()
			for _, s := range stmt.Catch.Body.Body {
				execNodeInContext(context, s, isTop, exports)
			}
		}()

		context.ExitContext()
	}

	// 执行 finally 块
	if stmt.Finally != nil {
		for _, s := range stmt.Finally.Body {
			execNodeInContext(context, s, isTop, exports)
		}
	}

	// 如果还有错误，继续抛出
	if hasError {
		panic(caughtErr)
	}
}

// execIfStatement 执行 if 语句
func execIfStatement(context *ProgramContext, stmt *types.IfStatement, isTop bool, exports map[string]*value.VsiVariable) {
	condition := evalExpression(context, stmt.Test)
	if isTruthy(condition) {
		for _, s := range stmt.Consequent.Body {
			execNodeInContext(context, s, isTop, exports)
		}
	} else if stmt.Alternate != nil {
		switch alt := stmt.Alternate.(type) {
		case *types.BlockStatement:
			for _, s := range alt.Body {
				execNodeInContext(context, s, isTop, exports)
			}
		case *types.IfStatement:
			execIfStatement(context, alt, isTop, exports)
		}
	}
}

// execForStatement 执行 for 循环
func execForStatement(context *ProgramContext, stmt *types.ForStatement, isTop bool, exports map[string]*value.VsiVariable) {
	// 初始化
	if stmt.Init != nil {
		execNodeInContext(context, stmt.Init, isTop, exports)
	}

	for {
		// 条件检查
		if stmt.Test != nil {
			condition := evalExpression(context, stmt.Test)
			if !isTruthy(condition) {
				break
			}
		}

		// 执行循环体
		for _, s := range stmt.Body.Body {
			execNodeInContext(context, s, isTop, exports)
		}

		// 更新
		if stmt.Update != nil {
			evalExpression(context, stmt.Update)
		}
	}
}

// execWhileStatement 执行 while 循环
func execWhileStatement(context *ProgramContext, stmt *types.WhileStatement, isTop bool, exports map[string]*value.VsiVariable) {
	for {
		condition := evalExpression(context, stmt.Test)
		if !isTruthy(condition) {
			break
		}
		for _, s := range stmt.Body.Body {
			execNodeInContext(context, s, isTop, exports)
		}
	}
}

// isTruthy 判断值是否为真
func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case value.VsiNumber:
		return val.Value != 0
	case *value.VsiNumber:
		return val.Value != 0
	case string:
		return len(val) > 0
	case value.VsiString:
		return len(val.Value) > 0
	case *value.VsiString:
		return len(val.Value) > 0
	default:
		return true
	}
}

// evalExpression 计算表达式
func evalExpression(context *ProgramContext, expr types.Expression) interface{} {
	switch expr.Type() {
	case "NumberLiteral":
		n := expr.(*types.NumberLiteral)
		var v int
		for i := 0; i < len(n.Value); i++ {
			ch := n.Value[i]
			if ch >= '0' && ch <= '9' {
				v = v*10 + int(ch-'0')
			}
		}
		return v
	case "StringLiteral":
		s := expr.(*types.StringLiteral)
		return s.Value
	case "Identifier":
		id := expr.(*types.Identifier)
		if v, ok := context.LookupVariable(id.Name); ok {
			return v.Value
		}
		if f, ok := context.LookupFunction(id.Name); ok {
			return f
		}
		return nil
	case "MemberExpression":
		return evalMemberExpression(context, expr.(*types.MemberExpression))
	case "CallExpression":
		return evalCallExpression(context, expr.(*types.CallExpression))
	case "ArrayExpression":
		arr := expr.(*types.ArrayExpression)
		items := []interface{}{}
		for _, e := range arr.Elements {
			items = append(items, evalExpression(context, e))
		}
		return value.CreateArray(items)
	case "BinaryExpression":
		return evalBinaryExpression(context, expr.(*types.BinaryExpression))
	case "NewExpression":
		return evalNewExpression(context, expr.(*types.NewExpression))
	default:
		return nil
	}
}

// evalMemberExpression 计算成员访问表达式
func evalMemberExpression(context *ProgramContext, m *types.MemberExpression) interface{} {
	obj := evalExpression(context, m.Object)

	// 处理 VsiObject
	if o, ok := obj.(*value.VsiObject); ok {
		if val, ok := o.Proto[m.Property]; ok {
			return val
		}
	}

	// 处理 VsiError
	if e, ok := obj.(*types.VsiError); ok {
		switch m.Property {
		case "Message":
			return e.Message
		case "ErrorType":
			return e.ErrorType
		case "Stack":
			items := []interface{}{}
			for _, frame := range e.Stack {
				items = append(items, frame.String())
			}
			return value.CreateArray(items)
		}
	}

	// 处理 VsiArray
	if arr, ok := obj.(*value.VsiArray); ok {
		if m.Property == "length" {
			return value.VsiNumber{Value: len(arr.Items)}
		}
		// 数字索引 - 解析字符串为数字
		idx := parseStringToInt(m.Property)
		if idx >= 0 && idx < len(arr.Items) {
			return arr.Items[idx]
		}
	}

	// 处理字符串
	switch v := obj.(type) {
	case string:
		if m.Property == "length" {
			return value.VsiNumber{Value: len([]rune(v))}
		}
		runes := []rune(v)
		idx := parseStringToInt(m.Property)
		if idx >= 0 && idx < len(runes) {
			return value.VsiString{Value: string(runes[idx])}
		}
	case value.VsiString:
		if m.Property == "length" {
			return value.VsiNumber{Value: len([]rune(v.Value))}
		}
		runes := []rune(v.Value)
		idx := parseStringToInt(m.Property)
		if idx >= 0 && idx < len(runes) {
			return value.VsiString{Value: string(runes[idx])}
		}
	case *value.VsiString:
		if m.Property == "length" {
			return value.VsiNumber{Value: len([]rune(v.Value))}
		}
		runes := []rune(v.Value)
		idx := parseStringToInt(m.Property)
		if idx >= 0 && idx < len(runes) {
			return value.VsiString{Value: string(runes[idx])}
		}
	}

	return nil
}

// parseStringToInt 解析字符串为整数
func parseStringToInt(s string) int {
	var idx int
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			idx = idx*10 + int(s[i]-'0')
		} else {
			return -1 // 不是纯数字
		}
	}
	return idx
}

// evalCallExpression 计算调用表达式
func evalCallExpression(context *ProgramContext, c *types.CallExpression) interface{} {
	callee := evalExpression(context, c.Callee)
	args := []interface{}{}
	for _, a := range c.Arguments {
		// 处理展开运算符
		if spread, ok := a.(*types.SpreadExpression); ok {
			spreadVal := evalExpression(context, spread.Argument)
			if arr, ok := spreadVal.(*value.VsiArray); ok {
				for _, item := range arr.Items {
					args = append(args, item)
				}
			}
		} else {
			args = append(args, evalExpression(context, a))
		}
	}

	// 如果是 VsiFunction，调用它
	if fn, ok := callee.(*value.VsiFunction); ok {
		res, err := fn.Call(args)
		if err != nil {
			panic(err)
		}
		return res
	}

	return nil
}

// evalBinaryExpression 计算二元表达式
func evalBinaryExpression(context *ProgramContext, b *types.BinaryExpression) interface{} {
	left := evalExpression(context, b.Left)
	right := evalExpression(context, b.Right)

	// 赋值
	if b.Operator == "=" {
		// 处理标识符赋值
		if id, ok := b.Left.(*types.Identifier); ok {
			if v, exists := context.LookupVariable(id.Name); exists {
				if v.Const {
					panic(fmt.Sprintf("Assignment to constant variable '%s'", id.Name))
				}
				v.Value = right
				return right
			}
			// 新变量
			context.SetVariable(id.Name, value.CreateVariable(right), false)
			return right
		}
		// 成员赋值
		if m, ok := b.Left.(*types.MemberExpression); ok {
			obj := evalExpression(context, m.Object)
			if o, ok := obj.(*value.VsiObject); ok {
				o.Proto[m.Property] = right
				return right
			}
			if arr, ok := obj.(*value.VsiArray); ok {
				idx := parseStringToInt(m.Property)
				if idx >= 0 && idx < len(arr.Items) {
					arr.Items[idx] = right
				}
				return right
			}
		}
		return nil
	}

	// 加法（字符串连接或数字加法）
	if b.Operator == "+" {
		var ls, rs string
		var lIsString, rIsString bool
		switch v := left.(type) {
		case string:
			ls = v
			lIsString = true
		case value.VsiString:
			ls = v.Value
			lIsString = true
		case *value.VsiString:
			ls = v.Value
			lIsString = true
		}
		switch v := right.(type) {
		case string:
			rs = v
			rIsString = true
		case value.VsiString:
			rs = v.Value
			rIsString = true
		case *value.VsiString:
			rs = v.Value
			rIsString = true
		}
		if lIsString || rIsString {
			return ls + rs
		}

		// 数字加法
		var li, ri int
		var lOk, rOk bool
		switch v := left.(type) {
		case int:
			li = v
			lOk = true
		case value.VsiNumber:
			li = v.Value
			lOk = true
		case *value.VsiNumber:
			li = v.Value
			lOk = true
		}
		switch v := right.(type) {
		case int:
			ri = v
			rOk = true
		case value.VsiNumber:
			ri = v.Value
			rOk = true
		case *value.VsiNumber:
			ri = v.Value
			rOk = true
		}
		if lOk && rOk {
			return li + ri
		}
		return nil
	}

	// 减法
	if b.Operator == "-" {
		var li, ri int
		switch v := left.(type) {
		case int:
			li = v
		case value.VsiNumber:
			li = v.Value
		case *value.VsiNumber:
			li = v.Value
		}
		switch v := right.(type) {
		case int:
			ri = v
		case value.VsiNumber:
			ri = v.Value
		case *value.VsiNumber:
			ri = v.Value
		}
		return li - ri
	}

	// 乘法
	if b.Operator == "*" {
		var li, ri int
		switch v := left.(type) {
		case int:
			li = v
		case value.VsiNumber:
			li = v.Value
		case *value.VsiNumber:
			li = v.Value
		}
		switch v := right.(type) {
		case int:
			ri = v
		case value.VsiNumber:
			ri = v.Value
		case *value.VsiNumber:
			ri = v.Value
		}
		return li * ri
	}

	// 除法
	if b.Operator == "/" {
		var li, ri int
		switch v := left.(type) {
		case int:
			li = v
		case value.VsiNumber:
			li = v.Value
		case *value.VsiNumber:
			li = v.Value
		}
		switch v := right.(type) {
		case int:
			ri = v
		case value.VsiNumber:
			ri = v.Value
		case *value.VsiNumber:
			ri = v.Value
		}
		if ri == 0 {
			panic("Division by zero")
		}
		return li / ri
	}

	return nil
}

// evalNewExpression 计算 new 表达式
func evalNewExpression(context *ProgramContext, n *types.NewExpression) interface{} {
	// 获取类名
	var className string
	switch c := n.Class.(type) {
	case *types.Identifier:
		className = c.Name
	default:
		panic("new expression requires a class name")
	}

	// 查找类定义
	class, ok := context.LookupClass(className)
	if !ok {
		panic(fmt.Sprintf("Class '%s' is not defined", className))
	}

	// 创建实例对象
	instance := value.CreateObject()
	instance.Proto["__class__"] = className

	// 执行构造函数
	args := []interface{}{}
	for _, a := range n.Arguments {
		args = append(args, evalExpression(context, a))
	}

	// 查找构造函数（名为 constructor 的方法或与类同名的方法）
	var constructor *types.ClassMethod = nil
	for i := range class.Methods {
		if class.Methods[i].Name == "constructor" || class.Methods[i].Name == className {
			constructor = &class.Methods[i]
			break
		}
	}

	// 执行构造函数
	if constructor != nil {
		// 创建局部作用域
		localCtx := context.CreateLocalContext()
		context.EnterContext(localCtx)

		// 绑定 this
		localCtx.Variables["this"] = value.CreateVariable(instance)

		// 绑定参数
		for i, p := range constructor.Params {
			var val interface{} = nil
			if i < len(args) {
				val = args[i]
			}
			localCtx.Variables[p.Name] = value.CreateVariable(val)
		}

		// 执行构造函数体
		func() {
			defer func() {
				if r := recover(); r != nil {
					if _, isReturn := r.(*ReturnPanic); !isReturn {
						panic(r)
					}
				}
			}()
			for _, stmt := range constructor.Body.Body {
				execNodeInContext(context, stmt, false, nil)
			}
		}()

		context.ExitContext()
	}

	// 绑定实例方法到实例对象
	for _, method := range class.Methods {
		if !method.IsStatic {
			methodCopy := method // capture
			instance.Proto[method.Name] = value.CreateFunction(method.Name, funcNames(method.Params), func(args []interface{}) (interface{}, error) {
				// 创建局部作用域
				localCtx := context.CreateLocalContext()
				context.EnterContext(localCtx)

				// 绑定 this
				localCtx.Variables["this"] = value.CreateVariable(instance)

				// 绑定参数
				for i, p := range methodCopy.Params {
					var val interface{} = nil
					if i < len(args) {
						val = args[i]
					}
					localCtx.Variables[p.Name] = value.CreateVariable(val)
				}

				// 压入调用栈
				frame := types.StackFrame{
					File:     context.CurrentFile,
					Function: className + "." + method.Name,
					Line:     0,
					Column:   0,
				}
				context.PushStack(frame)
				defer context.PopStack()

				// 执行方法体
				var result interface{} = nil
				func() {
					defer func() {
						if r := recover(); r != nil {
							if ret, ok := r.(*ReturnPanic); ok {
								result = ret.Value
							} else {
								panic(r)
							}
						}
					}()
					for _, stmt := range methodCopy.Body.Body {
						execNodeInContext(context, stmt, false, nil)
					}
				}()

				context.ExitContext()
				return result, nil
			})
		}
	}

	// 绑定静态方法到类对象（如果需要）
	// 这里可以创建一个类对象并绑定静态方法

	return instance
}

// helper to convert []types.Parameter to []string names
func funcNames(params []types.Parameter) []string {
	names := []string{}
	for _, p := range params {
		names = append(names, p.Name)
	}
	return names
}

// checkType 检查值是否符合类型
// 返回 true 表示类型匹配，false 表示不匹配
func checkType(val interface{}, typeStr string, context *ProgramContext) bool {
	if typeStr == "" {
		return true // 没有类型注解，不检查
	}

	// 查找类型对象
	typeVar, ok := context.LookupVariable(typeStr)
	if !ok {
		return true // 类型未定义，不检查
	}

	typeObj, ok := typeVar.Value.(*value.VsiObject)
	if !ok {
		return true
	}

	// 调用类型的 check 方法
	if checkFn, ok := typeObj.Proto["check"]; ok {
		if fn, ok := checkFn.(*value.VsiFunction); ok {
			result, err := fn.Call([]interface{}{val})
			if err != nil {
				return false
			}
			if b, ok := result.(bool); ok {
				return b
			}
		}
	}

	// 检查是否是类类型（用户定义的类）
	if class, ok := context.LookupClass(typeStr); ok {
		// 检查值是否是该类的实例
		if obj, ok := val.(*value.VsiObject); ok {
			if className, ok := obj.Proto["__class__"]; ok {
				if className == typeStr {
					return true
				}
				// 检查继承链
				return checkInheritance(className.(string), typeStr, class, context)
			}
		}
		return false
	}

	return true
}

// checkInheritance 检查继承关系
func checkInheritance(instanceClass, targetClass string, class *types.ClassDeclaration, context *ProgramContext) bool {
	if instanceClass == targetClass {
		return true
	}
	if class == nil || class.Parent == "" {
		return false
	}
	parentClass, ok := context.LookupClass(class.Parent)
	if !ok {
		return false
	}
	return checkInheritance(instanceClass, targetClass, parentClass, context)
}

// getTypeName 获取值的类型名称
func getTypeName(val interface{}) string {
	if val == nil {
		return "void"
	}
	switch val.(type) {
	case int:
		return "int"
	case value.VsiNumber:
		return "int"
	case *value.VsiNumber:
		return "int"
	case string:
		return "string"
	case value.VsiString:
		return "string"
	case *value.VsiString:
		return "string"
	case bool:
		return "bool"
	case *value.VsiArray:
		return "array"
	case *value.VsiObject:
		if className, ok := val.(*value.VsiObject).Proto["__class__"]; ok {
			return className.(string)
		}
		return "object"
	case *types.VsiError:
		return "Error"
	default:
		return "unknown"
	}
}
