package parser

import (
	"fmt"
	"strings"

	"github.com/RuanhoR/vsi/internal/compiler/tokenizr"
	"github.com/RuanhoR/vsi/internal/types"
)

// Parser 解析器结构
type Parser struct {
	tokens  []tokenizr.TokenData
	current int
}

// NewParser 创建新的解析器
func NewParser(tokens []tokenizr.TokenData) *Parser {
	return &Parser{
		tokens:  tokens,
		current: 0,
	}
}

// Parse 解析入口
func (p *Parser) Parse() *types.ProgramNode {
	program := &types.ProgramNode{
		Body: []types.BaseNode{},
	}

	for !p.isAtEnd() {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Body = append(program.Body, stmt)
		}
	}

	return program
}

// ==================== 辅助方法 ====================

// peek 查看当前 token
func (p *Parser) peek() tokenizr.TokenData {
	if p.current >= len(p.tokens) {
		return tokenizr.TokenData{Type: "EOF", Data: ""}
	}
	return p.tokens[p.current]
}

// advance 移动到下一个 token 并返回当前 token
func (p *Parser) advance() tokenizr.TokenData {
	if p.current >= len(p.tokens) {
		return tokenizr.TokenData{Type: "EOF", Data: ""}
	}
	token := p.tokens[p.current]
	p.current++
	return token
}

// isAtEnd 检查是否到达末尾
func (p *Parser) isAtEnd() bool {
	return p.current >= len(p.tokens)
}

// check 检查当前 token 类型
func (p *Parser) check(tokenType string) bool {
	return p.peek().Type == tokenType
}

// checkData 检查当前 token 数据
func (p *Parser) checkData(data string) bool {
	return p.peek().Data == data
}

// match 匹配并消费 token
func (p *Parser) match(tokenType string) bool {
	if p.check(tokenType) {
		p.advance()
		return true
	}
	return false
}

// matchData 匹配并消费特定数据的 token
func (p *Parser) matchData(data string) bool {
	if p.checkData(data) {
		p.advance()
		return true
	}
	return false
}

// expect 期望特定类型的 token
func (p *Parser) expect(tokenType string, message string) tokenizr.TokenData {
	if p.check(tokenType) {
		return p.advance()
	}
	panic(fmt.Sprintf("Parse error: %s, got %s", message, p.peek().Type))
}

// expectData 期望特定数据的 token
func (p *Parser) expectData(data string, message string) tokenizr.TokenData {
	if p.checkData(data) {
		return p.advance()
	}
	panic(fmt.Sprintf("Parse error: %s, got %s", message, p.peek().Data))
}

// ==================== 语句解析 ====================

// parseStatement 解析语句
func (p *Parser) parseStatement() types.Statement {
	// skip empty statement separators
	if p.check("SplitSymbol") {
		p.advance()
		return nil
	}
	// 检查关键字
	if p.check("Keyword") {
		switch p.peek().Data {
		case "fun":
			return p.parseFunctionDeclaration()
		case "var":
			return p.parseVarDefineDeclaration()
		case "const":
			return p.parseConstDefineDeclaration()
		case "import":
			return p.parseImportDeclaration()
		case "export":
			return p.parseExportDeclaration()
		case "if":
			return p.parseIfStatement()
		case "for":
			return p.parseForStatement()
		case "while":
			return p.parseWhileStatement()
		case "return":
			return p.parseReturnStatement()
		case "class":
			return p.parseClassDeclaration()
		case "throw":
			return p.parseThrowStatement()
		case "try":
			return p.parseTryStatement()
		}
	}

	// 表达式语句
	return p.parseExpressionStatement()
}

// parseFunctionDeclaration 解析函数声明
// fun name(param1: type1, param2: type2) { body }
func (p *Parser) parseFunctionDeclaration() *types.FunctionDeclaration {
	p.expectData("fun", "Expected 'fun' keyword")

	name := p.expect("Identifier", "Expected function name").Data

	p.expect("ParametersStart", "Expected '(' after function name")

	params := []types.Parameter{}
	if !p.check("ParametersEnd") {
		params = p.parseParameters()
	}

	p.expect("ParametersEnd", "Expected ')' after parameters")

	// 可选的返回类型
	returnType := ""
	if p.match("KeyNext") {
		returnType = p.expect("Identifier", "Expected return type").Data
	}

	body := p.parseBlock()

	return &types.FunctionDeclaration{
		Name:       name,
		Params:     params,
		Body:       body,
		ReturnType: returnType,
	}
}

// parseParameters 解析参数列表
func (p *Parser) parseParameters() []types.Parameter {
	params := []types.Parameter{}

	for {
		name := p.expect("Identifier", "Expected parameter name").Data
		paramType := ""

		// 可选的类型注解
		if p.match("KeyNext") {
			paramType = p.expect("Identifier", "Expected parameter type").Data
		}

		params = append(params, types.Parameter{
			Name: name,
			Type: paramType,
		})

		if !p.match("SplitSymbol") {
			break
		}
	}

	return params
}

// parseBlock 解析代码块
func (p *Parser) parseBlock() *types.BlockStatement {
	p.expect("BodyStart", "Expected '{'")

	statements := []types.Statement{}
	for !p.check("BodyEnd") && !p.isAtEnd() {
		stmt := p.parseStatement()
		if stmt != nil {
			statements = append(statements, stmt)
		}
	}

	p.expect("BodyEnd", "Expected '}'")

	return &types.BlockStatement{
		Body: statements,
	}
}

// parseVarDefineDeclaration 解析 var 声明
// var a = xxx, b = xxx;
func (p *Parser) parseVarDefineDeclaration() *types.VarDefineDeclaration {
	p.expectData("var", "Expected 'var' keyword")

	declarations := []types.VarDefine{}

	for {
		name := p.expect("Identifier", "Expected variable name").Data
		var value types.Expression = nil

		if p.match("Assignment") {
			value = p.parseExpression()
		}

		declarations = append(declarations, types.VarDefine{
			Name:  name,
			Value: value,
		})

		// If comma, continue parsing declarations in the same statement
		if p.matchData(",") {
			continue
		}
		// If semicolon, consume it and finish declaration
		if p.matchData(";") {
			break
		}
		// otherwise no separator, finish
		break
	}

	return &types.VarDefineDeclaration{
		Declarations: declarations,
	}
}

// parseConstDefineDeclaration 解析 const 声明
// const a = xxx, b = xxx;
func (p *Parser) parseConstDefineDeclaration() *types.ConstDefineDeclaration {
	p.expectData("const", "Expected 'const' keyword")

	declarations := []types.ConstDefine{}

	for {
		name := p.expect("Identifier", "Expected constant name").Data
		value := p.parseExpression()

		declarations = append(declarations, types.ConstDefine{
			Name:  name,
			Value: value,
		})

		// commas separate multiple consts, semicolon terminates statement
		if p.matchData(",") {
			continue
		}
		if p.matchData(";") {
			break
		}
		break
	}

	return &types.ConstDefineDeclaration{
		Declarations: declarations,
	}
}

// parseImportDeclaration 解析 import 声明
// import "path" as alias
func (p *Parser) parseImportDeclaration() *types.ImportDeclaration {
	p.expectData("import", "Expected 'import' keyword")

	// 解析字符串路径
	p.expect("StringStart", "Expected '\"' before import path")
	source := p.expect("StringContent", "Expected import path").Data
	p.expect("StringEnd", "Expected '\"' after import path")

	alias := ""
	if p.matchData("as") {
		alias = p.expect("Identifier", "Expected alias after 'as'").Data
	}

	return &types.ImportDeclaration{
		Source: source,
		Alias:  alias,
	}
}

// parseExportDeclaration 解析 export 声明
// export { aaa as a, bbb }
func (p *Parser) parseExportDeclaration() *types.ExportDeclaration {
	p.expectData("export", "Expected 'export' keyword")

	p.expect("BodyStart", "Expected '{' after 'export'")

	items := []types.ExportItem{}

	for {
		name := p.expect("Identifier", "Expected export name").Data
		alias := ""

		if p.matchData("as") {
			alias = p.expect("Identifier", "Expected alias after 'as'").Data
		}

		items = append(items, types.ExportItem{
			Name:  name,
			Alias: alias,
		})

		if !p.match("SplitSymbol") {
			break
		}
	}

	p.expect("BodyEnd", "Expected '}' after export items")

	return &types.ExportDeclaration{
		Items: items,
	}
}

// parseIfStatement 解析 if 语句
func (p *Parser) parseIfStatement() *types.IfStatement {
	p.expectData("if", "Expected 'if' keyword")
	p.expect("ParametersStart", "Expected '(' after 'if'")

	test := p.parseExpression()

	p.expect("ParametersEnd", "Expected ')' after condition")

	consequent := p.parseBlock()

	var alternate types.Statement = nil
	if p.matchData("else") {
		if p.checkData("if") {
			alternate = p.parseIfStatement()
		} else {
			alternate = p.parseBlock()
		}
	}

	return &types.IfStatement{
		Test:       test,
		Consequent: consequent,
		Alternate:  alternate,
	}
}

// parseForStatement 解析 for 语句
func (p *Parser) parseForStatement() *types.ForStatement {
	p.expectData("for", "Expected 'for' keyword")
	p.expect("ParametersStart", "Expected '(' after 'for'")

	var init types.Statement = nil
	if !p.check("SplitSymbol") {
		init = p.parseVarDefineDeclaration()
	}
	p.match("SplitSymbol")

	var test types.Expression = nil
	if !p.check("SplitSymbol") {
		test = p.parseExpression()
	}
	p.match("SplitSymbol")

	var update types.Expression = nil
	if !p.check("ParametersEnd") {
		update = p.parseExpression()
	}

	p.expect("ParametersEnd", "Expected ')' after for clauses")

	body := p.parseBlock()

	return &types.ForStatement{
		Init:   init,
		Test:   test,
		Update: update,
		Body:   body,
	}
}

// parseWhileStatement 解析 while 语句
func (p *Parser) parseWhileStatement() *types.WhileStatement {
	p.expectData("while", "Expected 'while' keyword")
	p.expect("ParametersStart", "Expected '(' after 'while'")

	test := p.parseExpression()

	p.expect("ParametersEnd", "Expected ')' after condition")

	body := p.parseBlock()

	return &types.WhileStatement{
		Test: test,
		Body: body,
	}
}

// parseReturnStatement 解析 return 语句
func (p *Parser) parseReturnStatement() *types.ReturnStatement {
	p.expectData("return", "Expected 'return' keyword")

	var argument types.Expression = nil
	if !p.check("BodyEnd") && !p.check("SplitSymbol") {
		argument = p.parseExpression()
	}

	return &types.ReturnStatement{
		Argument: argument,
	}
}

// parseExpressionStatement 解析表达式语句
func (p *Parser) parseExpressionStatement() *types.ExpressionStatement {
	expr := p.parseExpression()
	return &types.ExpressionStatement{
		Expression: expr,
	}
}

// ==================== 表达式解析 ====================

// parseExpression 解析表达式入口
func (p *Parser) parseExpression() types.Expression {
	return p.parseAssignment()
}

// parseAssignment 解析赋值表达式
func (p *Parser) parseAssignment() types.Expression {
	expr := p.parseBinary()

	if p.match("Assignment") {
		value := p.parseAssignment()
		return &types.BinaryExpression{
			Left:     expr,
			Operator: "=",
			Right:    value,
		}
	}

	return expr
}

// parseBinary 解析二元表达式
func (p *Parser) parseBinary() types.Expression {
	return p.parseBinaryWithPrecedence(0)
}

// 运算符优先级
var precedence = map[string]int{
	"+":  1,
	"-":  1,
	"*":  2,
	"/":  2,
	"==": 0,
	"!=": 0,
	"<":  0,
	"<=": 0,
	">":  0,
	">=": 0,
}

// parseBinaryWithPrecedence 使用优先级解析二元表达式
func (p *Parser) parseBinaryWithPrecedence(minPrec int) types.Expression {
	left := p.parseCall()

	for {
		op := p.peek().Data
		prec, ok := precedence[op]
		if !ok || prec < minPrec {
			break
		}

		if p.check("Operator") {
			p.advance()
			right := p.parseBinaryWithPrecedence(prec + 1)
			left = &types.BinaryExpression{
				Left:     left,
				Operator: op,
				Right:    right,
			}
		} else {
			break
		}
	}

	return left
}

// parseCall 解析调用表达式
func (p *Parser) parseCall() types.Expression {
	expr := p.parseMember()

	for {
		// 处理函数调用
		if p.match("ParametersStart") {
			args := []types.Expression{}
			if !p.check("ParametersEnd") {
				args = p.parseArguments()
			}
			p.expect("ParametersEnd", "Expected ')' after arguments")

			expr = &types.CallExpression{
				Callee:    expr,
				Arguments: args,
			}
		} else if p.match("ArrayStart") {
			// 处理数组索引访问 arr[0]
			index := p.parseExpression()
			p.expect("ArrayEnd", "Expected ']' after array index")

			// 将 arr[0] 转换为 MemberExpression
			// 索引作为属性名（字符串形式）
			expr = &types.MemberExpression{
				Object:   expr,
				Property: indexToString(index),
			}
		} else {
			break
		}
	}

	return expr
}

// indexToString 将索引表达式转换为字符串属性名
func indexToString(expr types.Expression) string {
	switch e := expr.(type) {
	case *types.NumberLiteral:
		return e.Value
	case *types.Identifier:
		return e.Name
	default:
		return ""
	}
}

// parseArguments 解析参数列表
func (p *Parser) parseArguments() []types.Expression {
	args := []types.Expression{}

	for {
		// 检查是否有展开运算符
		if p.check("SpreadOperator") {
			p.advance() // 消费 ...
			arg := p.parseExpression()
			args = append(args, &types.SpreadExpression{Argument: arg})
		} else {
			args = append(args, p.parseExpression())
		}

		if !p.match("SplitSymbol") {
			break
		}
	}

	return args
}

// parseMember 解析成员访问表达式
func (p *Parser) parseMember() types.Expression {
	// Inline primary parsing to avoid cases where parsePrimary sees a MemberSymbol
	// debug removed
	var expr types.Expression
	token := p.peek()
	switch token.Type {
	case "NumberLiteral":
		p.advance()
		expr = &types.NumberLiteral{Value: token.Data}
	case "StringStart":
		expr = p.parseStringLiteral()
	case "Identifier":
		p.advance()
		expr = &types.Identifier{Name: token.Data}
	case "ArrayStart":
		// parse array literal
		p.advance()
		elems := []types.Expression{}
		for !p.check("ArrayEnd") && !p.isAtEnd() {
			elems = append(elems, p.parseExpression())
			if !p.matchData(",") {
				break
			}
		}
		p.expect("ArrayEnd", "Expected ']' after array literal")
		expr = &types.ArrayExpression{Elements: elems}
	case "ParametersStart":
		p.advance()
		expr = p.parseExpression()
		p.expect("ParametersEnd", "Expected ')' after expression")
	case "Keyword":
		// Handle 'new' keyword
		if token.Data == "new" {
			expr = p.parseNewExpression()
		} else {
			panic(fmt.Sprintf("Unexpected keyword: %s", token.Data))
		}
	default:
		panic(fmt.Sprintf("Unexpected token: %s (%s)", token.Type, token.Data))
	}

	for p.match("MemberSymbol") {
		// 允许关键字作为属性名（如 Error.new）
		var property string
		if p.check("Identifier") {
			property = p.advance().Data
		} else if p.check("Keyword") {
			property = p.advance().Data
		} else {
			panic(fmt.Sprintf("Expected property name after '.', got %s (%s)", p.peek().Type, p.peek().Data))
		}
		expr = &types.MemberExpression{
			Object:   expr,
			Property: property,
		}
	}

	return expr
}

// parsePrimary 解析基本表达式
func (p *Parser) parsePrimary() types.Expression {
	token := p.peek()
	// debug removed

	switch token.Type {
	case "NumberLiteral":
		p.advance()
		return &types.NumberLiteral{Value: token.Data}

	case "StringStart":
		return p.parseStringLiteral()

	case "Identifier":
		p.advance()
		return &types.Identifier{Name: token.Data}

	case "ParametersStart":
		p.advance()
		expr := p.parseExpression()
		p.expect("ParametersEnd", "Expected ')' after expression")
		return expr
	default:
		panic(fmt.Sprintf("Unexpected token: %s (%s)", token.Type, token.Data))
	}
}

// parseStringLiteral 解析字符串字面量
func (p *Parser) parseStringLiteral() *types.StringLiteral {
	p.expect("StringStart", "Expected string start quote")

	content := ""
	if p.check("StringContent") {
		content = p.advance().Data
		// unescape common sequences
		content = strings.ReplaceAll(content, "\\n", "\n")
		content = strings.ReplaceAll(content, "\\t", "\t")
		content = strings.ReplaceAll(content, "\\r", "\r")
		content = strings.ReplaceAll(content, "\\\"", "\"")
		content = strings.ReplaceAll(content, "\\\\", "\\")
	}

	p.expect("StringEnd", "Expected string end quote")

	return &types.StringLiteral{Value: content}
}

// ==================== 公共解析函数 ====================

// ParseTokens 从 token 列表解析 AST
func ParseTokens(tokens []tokenizr.TokenData) *types.ProgramNode {
	parser := NewParser(tokens)
	return parser.Parse()
}
func ParseString(str string) *types.ProgramNode {
	tokens := tokenizr.GenerateTokenizr(str)
	parser := NewParser(tokens)
	return parser.Parse()
}

// ==================== Class 相关解析 ====================

// parseClassDeclaration 解析类声明
// class Name { public static xxx() {} public xxx() {} }
func (p *Parser) parseClassDeclaration() *types.ClassDeclaration {
	p.expectData("class", "Expected 'class' keyword")

	name := p.expect("Identifier", "Expected class name").Data

	var parent string = ""
	if p.matchData("extends") {
		parent = p.expect("Identifier", "Expected parent class name").Data
	}

	p.expect("BodyStart", "Expected '{' after class name")

	methods := []types.ClassMethod{}
	properties := []types.ClassProperty{}

	for !p.check("BodyEnd") && !p.isAtEnd() {
		isPublic := false
		isStatic := false

		// 解析修饰符
		if p.matchData("public") {
			isPublic = true
		}
		if p.matchData("static") {
			isStatic = true
		}
		// public static 或 static public 顺序
		if p.matchData("public") {
			isPublic = true
		}
		if p.matchData("static") {
			isStatic = true
		}

		// 方法或属性
		if p.checkData("fun") {
			// 方法
			p.advance() // consume 'fun'
			methodName := p.expect("Identifier", "Expected method name").Data

			p.expect("ParametersStart", "Expected '(' after method name")
			params := []types.Parameter{}
			if !p.check("ParametersEnd") {
				params = p.parseParameters()
			}
			p.expect("ParametersEnd", "Expected ')' after parameters")

			returnType := ""
			if p.match("KeyNext") {
				returnType = p.expect("Identifier", "Expected return type").Data
			}

			body := p.parseBlock()

			methods = append(methods, types.ClassMethod{
				Name:       methodName,
				Params:     params,
				Body:       body,
				IsStatic:   isStatic,
				IsPublic:   isPublic,
				ReturnType: returnType,
			})
		} else if p.check("Identifier") {
			// 属性
			propName := p.advance().Data
			var value types.Expression = nil
			if p.match("Assignment") {
				value = p.parseExpression()
			}
			p.match("SplitSymbol") // 可选的分号

			properties = append(properties, types.ClassProperty{
				Name:     propName,
				Value:    value,
				IsStatic: isStatic,
				IsPublic: isPublic,
			})
		} else {
			panic(fmt.Sprintf("Unexpected token in class body: %s (%s)", p.peek().Type, p.peek().Data))
		}
	}

	p.expect("BodyEnd", "Expected '}' after class body")

	return &types.ClassDeclaration{
		Name:       name,
		Methods:    methods,
		Properties: properties,
		Parent:     parent,
	}
}

// ==================== 错误处理解析 ====================

// parseThrowStatement 解析 throw 语句
func (p *Parser) parseThrowStatement() *types.ThrowStatement {
	p.expectData("throw", "Expected 'throw' keyword")

	argument := p.parseExpression()

	return &types.ThrowStatement{
		Argument: argument,
	}
}

// parseTryStatement 解析 try 语句
func (p *Parser) parseTryStatement() *types.TryStatement {
	p.expectData("try", "Expected 'try' keyword")

	block := p.parseBlock()

	var catch *types.CatchClause = nil
	if p.matchData("catch") {
		p.expect("ParametersStart", "Expected '(' after 'catch'")
		param := p.expect("Identifier", "Expected error variable name").Data
		p.expect("ParametersEnd", "Expected ')' after error variable")
		catchBody := p.parseBlock()
		catch = &types.CatchClause{
			Param: param,
			Body:  catchBody,
		}
	}

	var finally *types.BlockStatement = nil
	if p.matchData("finally") {
		finally = p.parseBlock()
	}

	return &types.TryStatement{
		Block:   block,
		Catch:   catch,
		Finally: finally,
	}
}

// parseNewExpression 解析 new 表达式
func (p *Parser) parseNewExpression() *types.NewExpression {
	p.expectData("new", "Expected 'new' keyword")

	classExpr := p.parseMember()

	p.expect("ParametersStart", "Expected '(' after class name")
	args := []types.Expression{}
	if !p.check("ParametersEnd") {
		args = p.parseArguments()
	}
	p.expect("ParametersEnd", "Expected ')' after arguments")

	return &types.NewExpression{
		Class:     classExpr,
		Arguments: args,
	}
}
