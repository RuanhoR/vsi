package run

import (
	"math"

	"github.com/RuanhoR/vsi/internal/runner/value"
	"github.com/RuanhoR/vsi/internal/types"
)

// RunNode executes a Program AST node and returns module exports.
func RunNode(node types.ProgramNode) map[string]*value.VsiVariable {
	Type := node.Type()
	if Type != "Program" {
		panic("Expected Program Node")
	}
	context := createContext()
	exports := make(map[string]*value.VsiVariable)
	for i := range node.Body {
		execNodeInContext(context, node.Body[i], true, exports)
	}
	return exports
}

func execNodeInContext(context *types.ProgramContext, node types.BaseNode, isTop bool, exports map[string]*value.VsiVariable) {
	switch node.Type() {
	case "FunctionDeclaration":
		decl := node.(*types.FunctionDeclaration)
		// create a VsiFunction which when called will execute the function body
		fn := value.CreateFunction(decl.Name, funcNames(decl.Params), func(args []interface{}) (interface{}, error) {
			// create a new context for function execution
			newCtx := &types.ProgramContext{
				Top: context.Top,
				Current: &types.Context{
					Variables: make(map[string]*value.VsiVariable),
					Functions: make(map[string]*value.VsiFunction),
					Imports:   make(map[string]interface{}),
				},
			}
			// copy parent current variables as initial scope (simple closure/read access)
			for k, v := range context.Current.Variables {
				newCtx.Current.Variables[k] = v
			}
			// bind parameters
			for i, p := range decl.Params {
				var val interface{} = nil
				if i < len(args) {
					val = args[i]
				}
				newCtx.Current.Variables[p.Name] = value.CreateVariable(val)
			}
			// execute function body statements
			for _, stmt := range decl.Body.Body {
				// handle return directly here
				if stmt.Type() == "ReturnStatement" {
					ret := stmt.(*types.ReturnStatement)
					if ret.Argument == nil {
						return nil, nil
					}
					return evalExpression(newCtx, ret.Argument), nil
				}
				execNodeInContext(newCtx, stmt, false, exports)
			}
			return nil, nil
		})
		v := value.CreateVariable(fn)
		if isTop {
			context.Top.Variables[decl.Name] = v
		} else {
			context.Current.Variables[decl.Name] = v
		}
	case "VarDefineDeclaration":
		decl := node.(*types.VarDefineDeclaration)
		// batch write according to level: top-level -> Top, otherwise Current
		for _, d := range decl.Declarations {
			varVal := interface{}(nil)
			if d.Value != nil {
				varVal = evalExpression(context, d.Value)
			}
			v := value.CreateVariable(varVal)
			if isTop {
				context.Top.Variables[d.Name] = v
			} else {
				context.Current.Variables[d.Name] = v
			}
		}
	case "ConstDefineDeclaration":
		decl := node.(*types.ConstDefineDeclaration)
		for _, d := range decl.Declarations {
			varVal := interface{}(nil)
			if d.Value != nil {
				varVal = evalExpression(context, d.Value)
			}
			v := value.CreateVariable(varVal)
			v.Const = true
			if isTop {
				context.Top.Variables[d.Name] = v
			} else {
				context.Current.Variables[d.Name] = v
			}
		}
	case "ExportDeclaration":
		decl := node.(*types.ExportDeclaration)
		for _, item := range decl.Items {
			// find variable in current then top
			if vv, ok := context.Current.Variables[item.Name]; ok {
				exports[item.Name] = vv
			} else if vv, ok := context.Top.Variables[item.Name]; ok {
				exports[item.Name] = vv
			}
			// if alias present, mirror
			if item.Alias != "" {
				if e, ok := exports[item.Name]; ok {
					exports[item.Alias] = e
				}
			}
		}
	case "ExpressionStatement":
		stmt := node.(*types.ExpressionStatement)
		// evaluate expression (calls will execute via evalExpression)
		_ = evalExpression(context, stmt.Expression)
	default:
		// other node types not yet implemented
	}
}

func evalExpression(context *types.ProgramContext, expr types.Expression) interface{} {
	switch expr.Type() {
	case "NumberLiteral":
		n := expr.(*types.NumberLiteral)
		// try to convert to int
		// simple atoi-like conversion
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
		if v, ok := context.Current.Variables[id.Name]; ok {
			return v.Value
		}
		if v, ok := context.Top.Variables[id.Name]; ok {
			return v.Value
		}
		return nil
	case "MemberExpression":
		m := expr.(*types.MemberExpression)
		obj := evalExpression(context, m.Object)
		// only support VsiObject with Proto for now
		if o, ok := obj.(*value.VsiObject); ok {
			if val, ok := o.Proto[m.Property]; ok {
				return val
			}
		}
		return nil
	case "CallExpression":
		c := expr.(*types.CallExpression)
		callee := evalExpression(context, c.Callee)
		args := []interface{}{}
		for _, a := range c.Arguments {
			args = append(args, evalExpression(context, a))
		}
		// if callee is VsiFunction, call it
		if fn, ok := callee.(*value.VsiFunction); ok {
			res, err := fn.Call(args)
			if err != nil {
				panic(err)
			}
			return res
		}
		return nil
	case "ArrayExpression":
		arr := expr.(*types.ArrayExpression)
		items := []interface{}{}
		for _, e := range arr.Elements {
			items = append(items, evalExpression(context, e))
		}
		return value.CreateArray(items)
	case "BinaryExpression":
		b := expr.(*types.BinaryExpression)
		left := evalExpression(context, b.Left)
		right := evalExpression(context, b.Right)
		// support + for string concatenation and numeric addition
		if b.Operator == "+" {
			// try string concatenation first
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
			// fallback to integer addition when possible
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
		return nil
	default:
		return nil
	}
}

// helper to convert []types.Parameter to []string names
func funcNames(params []types.Parameter) []string {
	names := []string{}
	for _, p := range params {
		names = append(names, p.Name)
	}
	return names
}

// vsiToNative converts Vsi runtime values into native Go values suitable for json.Marshal
func vsiToNative(v interface{}) interface{} {
	switch t := v.(type) {
	case *value.VsiObject:
		m := make(map[string]interface{})
		for k, vv := range t.Proto {
			m[k] = vsiToNative(vv)
		}
		return m
	case *value.VsiArray:
		arr := []interface{}{}
		for _, it := range t.Items {
			arr = append(arr, vsiToNative(it))
		}
		return arr
	case value.VsiString:
		return t.Value
	case *value.VsiString:
		return t.Value
	case value.VsiNumber:
		return t.Value
	case *value.VsiNumber:
		return t.Value
	default:
		return v
	}
}

// nativeToVsi converts decoded json (map[string]interface{}, []interface{}, float64, string, bool, nil)
// into Vsi runtime values (VsiObject, VsiArray, int, string, bool, nil)
func nativeToVsi(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		o := value.CreateObject()
		for k, vv := range t {
			o.Proto[k] = nativeToVsi(vv)
		}
		return o
	case []interface{}:
		items := []interface{}{}
		for _, it := range t {
			items = append(items, nativeToVsi(it))
		}
		return value.CreateArray(items)
	case float64:
		// convert to int when integral
		if math.Mod(t, 1) == 0 {
			return int(t)
		}
		return t
	default:
		return v
	}
}
