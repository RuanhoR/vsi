package types

import (
	"fmt"

	"github.com/RuanhoR/vsi/internal/runner/value"
)

// ==================== 基础接口 ====================

// BaseNode AST 节点基础接口，所有 AST 节点都必须实现
type BaseNode interface {
	Type() string
}

// Expression 表达式接口
type Expression interface {
	BaseNode
	expressionNode()
}

// Statement 语句接口
type Statement interface {
	BaseNode
	statementNode()
}

// Declaration 声明接口
type Declaration interface {
	Statement
	declarationNode()
}

// ==================== 程序节点 ====================

// ProgramNode 程序根节点
type ProgramNode struct {
	Body []BaseNode
}

func (p *ProgramNode) Type() string   { return "Program" }
func (p *ProgramNode) statementNode() {}
func (p *ProgramNode) String() string {
	return fmt.Sprintf("ProgramNode{Body: %v}", p.Body)
}

// ==================== 字面量 ====================

// NumberLiteral 数字字面量
type NumberLiteral struct {
	Value string
}

func (n *NumberLiteral) Type() string    { return "NumberLiteral" }
func (n *NumberLiteral) expressionNode() {}
func (n *NumberLiteral) String() string {
	return fmt.Sprintf("NumberLiteral{Value: %q}", n.Value)
}

// StringLiteral 字符串字面量
type StringLiteral struct {
	Value string
}

func (s *StringLiteral) Type() string    { return "StringLiteral" }
func (s *StringLiteral) expressionNode() {}
func (s *StringLiteral) String() string {
	return fmt.Sprintf("StringLiteral{Value: %q}", s.Value)
}

// ==================== 标识符 ====================

// Identifier 标识符
type Identifier struct {
	Name string
}

func (i *Identifier) Type() string    { return "Identifier" }
func (i *Identifier) expressionNode() {}
func (i *Identifier) String() string {
	return fmt.Sprintf("Identifier{Name: %q}", i.Name)
}

// ==================== 表达式 ====================

// MemberExpression 成员访问表达式 (如: a.b, a.b.c)
type MemberExpression struct {
	Object   Expression // 对象部分
	Property string     // 属性名
}

func (m *MemberExpression) Type() string    { return "MemberExpression" }
func (m *MemberExpression) expressionNode() {}
func (m *MemberExpression) String() string {
	return fmt.Sprintf("MemberExpression{Object: %v, Property: %q}", m.Object, m.Property)
}

// CallExpression 函数调用表达式
type CallExpression struct {
	Callee    Expression   // 被调用的函数
	Arguments []Expression // 参数列表
}

func (c *CallExpression) Type() string    { return "CallExpression" }
func (c *CallExpression) expressionNode() {}
func (c *CallExpression) String() string {
	return fmt.Sprintf("CallExpression{Callee: %v, Arguments: %v}", c.Callee, c.Arguments)
}

// ArrayExpression 数组字面量
type ArrayExpression struct {
	Elements []Expression
}

func (a *ArrayExpression) Type() string    { return "ArrayExpression" }
func (a *ArrayExpression) expressionNode() {}
func (a *ArrayExpression) String() string {
	return fmt.Sprintf("ArrayExpression{Elements: %v}", a.Elements)
}

// BinaryExpression 二元表达式
type BinaryExpression struct {
	Left     Expression
	Operator string
	Right    Expression
}

func (b *BinaryExpression) Type() string    { return "BinaryExpression" }
func (b *BinaryExpression) expressionNode() {}
func (b *BinaryExpression) String() string {
	return fmt.Sprintf("BinaryExpression{Left: %v, Operator: %q, Right: %v}", b.Left, b.Operator, b.Right)
}

// ==================== 语句 ====================

// BlockStatement 代码块语句
type BlockStatement struct {
	Body []Statement
}

func (b *BlockStatement) Type() string   { return "BlockStatement" }
func (b *BlockStatement) statementNode() {}
func (b *BlockStatement) String() string {
	return fmt.Sprintf("BlockStatement{Body: %v}", b.Body)
}

// ExpressionStatement 表达式语句
type ExpressionStatement struct {
	Expression Expression
}

func (e *ExpressionStatement) Type() string   { return "ExpressionStatement" }
func (e *ExpressionStatement) statementNode() {}
func (e *ExpressionStatement) String() string {
	return fmt.Sprintf("ExpressionStatement{Expression: %v}", e.Expression)
}

// ReturnStatement return 语句
type ReturnStatement struct {
	Argument Expression
}

func (r *ReturnStatement) Type() string   { return "ReturnStatement" }
func (r *ReturnStatement) statementNode() {}
func (r *ReturnStatement) String() string {
	return fmt.Sprintf("ReturnStatement{Argument: %v}", r.Argument)
}

// ==================== 声明 ====================

// Parameter 函数参数
type Parameter struct {
	Name string
	Type string // 参数类型，可能为空
}

func (p Parameter) String() string {
	return fmt.Sprintf("Parameter{Name: %q, Type: %q}", p.Name, p.Type)
}

// FunctionDeclaration 函数声明
type FunctionDeclaration struct {
	Name       string
	Params     []Parameter
	Body       *BlockStatement
	ReturnType string // 返回类型，可能为空
}

func (f *FunctionDeclaration) Type() string     { return "FunctionDeclaration" }
func (f *FunctionDeclaration) statementNode()   {}
func (f *FunctionDeclaration) declarationNode() {}
func (f *FunctionDeclaration) String() string {
	return fmt.Sprintf("FunctionDeclaration{Name: %q, Params: %v, Body: %v, ReturnType: %q}", f.Name, f.Params, f.Body, f.ReturnType)
}

// VarDefine 单个变量定义 (用于 var a = xxx, b = xxx 结构)
type VarDefine struct {
	Name  string
	Value Expression // 初始值，可能为空
}

func (v *VarDefine) Type() string { return "VarDefine" }
func (v *VarDefine) String() string {
	return fmt.Sprintf("VarDefine{Name: %q, Value: %v}", v.Name, v.Value)
}

// VarDefineDeclaration var 定义声明
type VarDefineDeclaration struct {
	Declarations []VarDefine
}

func (v *VarDefineDeclaration) Type() string     { return "VarDefineDeclaration" }
func (v *VarDefineDeclaration) statementNode()   {}
func (v *VarDefineDeclaration) declarationNode() {}
func (v *VarDefineDeclaration) String() string {
	return fmt.Sprintf("VarDefineDeclaration{Declarations: %v}", v.Declarations)
}

// ImportDeclaration import 导入声明
type ImportDeclaration struct {
	Source string // 导入的文件路径
	Alias  string // 别名
}

func (i *ImportDeclaration) Type() string     { return "ImportDeclaration" }
func (i *ImportDeclaration) statementNode()   {}
func (i *ImportDeclaration) declarationNode() {}
func (i *ImportDeclaration) String() string {
	return fmt.Sprintf("ImportDeclaration{Source: %q, Alias: %q}", i.Source, i.Alias)
}

// ExportItem 导出项
type ExportItem struct {
	Name  string // 原名称
	Alias string // 导出别名，如果与原名相同则为空
}

func (e ExportItem) String() string {
	return fmt.Sprintf("ExportItem{Name: %q, Alias: %q}", e.Name, e.Alias)
}

// ExportDeclaration export 导出声明
type ExportDeclaration struct {
	Items []ExportItem
}

func (e *ExportDeclaration) Type() string     { return "ExportDeclaration" }
func (e *ExportDeclaration) statementNode()   {}
func (e *ExportDeclaration) declarationNode() {}
func (e *ExportDeclaration) String() string {
	return fmt.Sprintf("ExportDeclaration{Items: %v}", e.Items)
}

// ConstDefine 单个常量定义
type ConstDefine struct {
	Name  string
	Value Expression
}

func (c *ConstDefine) Type() string { return "ConstDefine" }
func (c *ConstDefine) String() string {
	return fmt.Sprintf("ConstDefine{Name: %q, Value: %v}", c.Name, c.Value)
}

// ConstDefineDeclaration const 定义声明
type ConstDefineDeclaration struct {
	Declarations []ConstDefine
}

func (c *ConstDefineDeclaration) Type() string     { return "ConstDefineDeclaration" }
func (c *ConstDefineDeclaration) statementNode()   {}
func (c *ConstDefineDeclaration) declarationNode() {}
func (c *ConstDefineDeclaration) String() string {
	return fmt.Sprintf("ConstDefineDeclaration{Declarations: %v}", c.Declarations)
}

// IfStatement if 语句
type IfStatement struct {
	Test       Expression
	Consequent *BlockStatement
	Alternate  Statement // else 分支，可能为空
}

func (i *IfStatement) Type() string   { return "IfStatement" }
func (i *IfStatement) statementNode() {}
func (i *IfStatement) String() string {
	return fmt.Sprintf("IfStatement{Test: %v, Consequent: %v, Alternate: %v}", i.Test, i.Consequent, i.Alternate)
}

// ForStatement for 循环
type ForStatement struct {
	Init   Statement
	Test   Expression
	Update Expression
	Body   *BlockStatement
}

func (f *ForStatement) Type() string   { return "ForStatement" }
func (f *ForStatement) statementNode() {}
func (f *ForStatement) String() string {
	return fmt.Sprintf("ForStatement{Init: %v, Test: %v, Update: %v, Body: %v}", f.Init, f.Test, f.Update, f.Body)
}

// WhileStatement while 循环
type WhileStatement struct {
	Test Expression
	Body *BlockStatement
}

func (w *WhileStatement) Type() string   { return "WhileStatement" }
func (w *WhileStatement) statementNode() {}
func (w *WhileStatement) String() string {
	return fmt.Sprintf("WhileStatement{Test: %v, Body: %v}", w.Test, w.Body)
}

type Context struct {
	Variables map[string]*value.VsiVariable
	Functions map[string]*value.VsiFunction
	Imports   map[string]interface{}
}
type ProgramContext struct {
	Top     *Context
	Current *Context
}
