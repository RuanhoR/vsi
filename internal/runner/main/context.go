package run

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/RuanhoR/vsi/internal/runner/value"
	"github.com/RuanhoR/vsi/internal/types"
	"github.com/RuanhoR/vsi/pkg/config"
)

// ==================== 类型转换函数 ====================

// VsiToNative 将 Vsi 运行时值转换为原生 Go 值
func VsiToNative(v interface{}) interface{} {
	switch t := v.(type) {
	case *value.VsiObject:
		m := make(map[string]interface{})
		for k, vv := range t.Proto {
			m[k] = VsiToNative(vv)
		}
		return m
	case *value.VsiArray:
		arr := []interface{}{}
		for _, it := range t.Items {
			arr = append(arr, VsiToNative(it))
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
	case *types.VsiError:
		return fmt.Sprintf("%s: %s", t.ErrorType, t.Message)
	default:
		return v
	}
}

// NativeToVsi 将原生 Go 值转换为 Vsi 运行时值
func NativeToVsi(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		o := value.CreateObject()
		for k, vv := range t {
			o.Proto[k] = NativeToVsi(vv)
		}
		return o
	case []interface{}:
		items := []interface{}{}
		for _, it := range t {
			items = append(items, NativeToVsi(it))
		}
		return value.CreateArray(items)
	case float64:
		// convert to int when integral
		if math.Mod(t, 1) == 0 {
			return int(t)
		}
		return t
	case string:
		return value.VsiString{Value: t}
	case int:
		return value.VsiNumber{Value: t}
	case bool:
		return t
	default:
		return v
	}
}

// ==================== 上下文层级 ====================

// ContextLevel 表示上下文层级
type ContextLevel int

const (
	LevelGlobal ContextLevel = iota // 全局运行时
	LevelFile                       // 文件全局
	LevelLocal                      // 局部作用域
)

// Context 运行时上下文
type Context struct {
	Variables map[string]*value.VsiVariable
	Functions map[string]*value.VsiFunction
	Imports   map[string]interface{}
	Classes   map[string]*types.ClassDeclaration // 类定义
	Parent    *Context                           // 父上下文
	Level     ContextLevel                       // 上下文层级
}

// ProgramContext 程序运行时上下文
type ProgramContext struct {
	Global      *Context           // 全局运行时（内置对象、类型构造函数）
	File        *Context           // 文件全局（文件级别的变量、函数）
	Current     *Context           // 当前作用域（可能是 Global、File 或 Local）
	Stack       []types.StackFrame // 调用栈
	CurrentFile string             // 当前执行的文件名
}

// NewContext 创建新上下文
func NewContext(level ContextLevel, parent *Context) *Context {
	return &Context{
		Variables: make(map[string]*value.VsiVariable),
		Functions: make(map[string]*value.VsiFunction),
		Imports:   make(map[string]interface{}),
		Classes:   make(map[string]*types.ClassDeclaration),
		Parent:    parent,
		Level:     level,
	}
}

// LookupVariable 在当前上下文及父上下文中查找变量
func (c *Context) LookupVariable(name string) (*value.VsiVariable, bool) {
	if v, ok := c.Variables[name]; ok {
		return v, true
	}
	if c.Parent != nil {
		return c.Parent.LookupVariable(name)
	}
	return nil, false
}

// LookupFunction 在当前上下文及父上下文中查找函数
func (c *Context) LookupFunction(name string) (*value.VsiFunction, bool) {
	if f, ok := c.Functions[name]; ok {
		return f, true
	}
	if c.Parent != nil {
		return c.Parent.LookupFunction(name)
	}
	return nil, false
}

// LookupClass 在当前上下文及父上下文中查找类
func (c *Context) LookupClass(name string) (*types.ClassDeclaration, bool) {
	if cls, ok := c.Classes[name]; ok {
		return cls, true
	}
	if c.Parent != nil {
		return c.Parent.LookupClass(name)
	}
	return nil, false
}

// ==================== 全局运行时创建 ====================

// createGlobalContext 创建全局运行时上下文
func createGlobalContext() *Context {
	global := NewContext(LevelGlobal, nil)

	// ==================== process 对象 ====================
	process := value.CreateObject()

	// env: 环境变量
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}
	process.Proto["env"] = envMap
	process.Proto["argv"] = os.Args

	// version
	processVersion := value.CreateObject()
	processVersion.Proto["vsi"] = config.Version
	processVersion.Proto["go"] = runtime.Version()
	process.Proto["version"] = processVersion
	process.Proto["pid"] = os.Getpid()
	process.Proto["platform"] = value.VsiString{Value: runtime.GOOS}
	process.Proto["arch"] = value.VsiString{Value: runtime.GOARCH}

	home, _ := os.Getwd()
	process.Proto["cwd"] = value.VsiString{Value: home}

	// stdout
	stdout := value.CreateObject()
	stdout.Proto["write"] = value.CreateFunction("write", []string{"data"}, func(args []interface{}) (interface{}, error) {
		if len(args) > 0 {
			switch v := args[0].(type) {
			case string:
				os.Stdout.WriteString(v)
			case int:
				os.Stdout.WriteString(fmt.Sprint(v))
			case value.VsiString:
				os.Stdout.WriteString(v.Value)
			case *value.VsiString:
				os.Stdout.WriteString(v.Value)
			case value.VsiNumber:
				os.Stdout.WriteString(fmt.Sprint(v.Value))
			case *value.VsiNumber:
				os.Stdout.WriteString(fmt.Sprint(v.Value))
			case *types.VsiError:
				os.Stdout.WriteString(fmt.Sprintf("%s: %s", v.ErrorType, v.Message))
			case *value.VsiArray:
				os.Stdout.WriteString(fmt.Sprintf("%v", v.Items))
			case *value.VsiObject:
				os.Stdout.WriteString(fmt.Sprintf("%v", v.Proto))
			default:
				os.Stdout.WriteString(fmt.Sprint(v))
			}
		}
		return nil, nil
	})
	process.Proto["stdout"] = stdout

	// stderr
	stderr := value.CreateObject()
	stderr.Proto["write"] = value.CreateFunction("write", []string{"data"}, func(args []interface{}) (interface{}, error) {
		if len(args) > 0 {
			switch v := args[0].(type) {
			case string:
				os.Stderr.WriteString(v)
			case int:
				os.Stderr.WriteString(fmt.Sprint(v))
			default:
				os.Stderr.WriteString(fmt.Sprint(v))
			}
		}
		return nil, nil
	})
	process.Proto["stderr"] = stderr

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
		resultArr := []int{}
		for _, data := range result {
			resultArr = append(resultArr, int(data))
		}
		resultIface := make([]interface{}, len(resultArr))
		for i, v := range resultArr {
			resultIface[i] = v
		}
		return value.CreateArray(resultIface), nil
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
		return filepath.Join(parts...), nil
	})
	pathObj.Proto["dirname"] = value.CreateFunction("dirname", []string{"p"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return "", nil
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
		return filepath.Dir(p), nil
	})
	pathObj.Proto["basename"] = value.CreateFunction("basename", []string{"p"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return "", nil
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
		return filepath.Base(p), nil
	})
	pathObj.Proto["extname"] = value.CreateFunction("extname", []string{"p"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return "", nil
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
		return filepath.Ext(p), nil
	})
	pathObj.Proto["isAbs"] = value.CreateFunction("isAbs", []string{"p"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return false, nil
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
		return filepath.IsAbs(p), nil
	})
	pathObj.Proto["resolve"] = value.CreateFunction("resolve", []string{"...paths"}, func(args []interface{}) (interface{}, error) {
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
		return filepath.Clean(filepath.Join(parts...)), nil
	})
	process.Proto["path"] = pathObj

	// console 对象
	consoleObj := value.CreateObject()
	consoleObj.Proto["log"] = value.CreateFunction("log", []string{"...data"}, func(args []interface{}) (interface{}, error) {
		for _, arg := range args {
			switch v := arg.(type) {
			case string:
				fmt.Println(v)
			case int:
				fmt.Println(fmt.Sprint(v))
			case value.VsiString:
				fmt.Println(v.Value)
			case *value.VsiString:
				fmt.Println(v.Value)
			case value.VsiNumber:
				fmt.Println(v.Value)
			case *value.VsiNumber:
				fmt.Println(v.Value)
			default:
				fmt.Println(fmt.Sprint(v))
			}
		}
		return nil, nil
	})
	consoleObj.Proto["error"] = value.CreateFunction("error", []string{"...data"}, func(args []interface{}) (interface{}, error) {
		for _, arg := range args {
			switch v := arg.(type) {
			case string:
				fmt.Fprintln(os.Stderr, v)
			default:
				fmt.Fprintln(os.Stderr, fmt.Sprint(v))
			}
		}
		return nil, nil
	})
	process.Proto["console"] = consoleObj

	// net.http 对象
	httpObj := value.CreateObject()
	// HTTP methods
	httpMethods := value.CreateArray([]interface{}{
		value.VsiString{Value: "GET"},
		value.VsiString{Value: "POST"},
		value.VsiString{Value: "PUT"},
		value.VsiString{Value: "DELETE"},
		value.VsiString{Value: "PATCH"},
		value.VsiString{Value: "HEAD"},
		value.VsiString{Value: "OPTIONS"},
	})
	httpObj.Proto["methods"] = httpMethods
	// HTTP status codes
	statusObj := value.CreateObject()
	statusObj.Proto["OK"] = 200
	statusObj.Proto["Created"] = 201
	statusObj.Proto["NoContent"] = 204
	statusObj.Proto["BadRequest"] = 400
	statusObj.Proto["Unauthorized"] = 401
	statusObj.Proto["Forbidden"] = 403
	statusObj.Proto["NotFound"] = 404
	statusObj.Proto["InternalServerError"] = 500
	httpObj.Proto["status"] = statusObj
	// ResponseInit 构造函数
	responseInitObj := value.CreateObject()
	responseInitObj.Proto["new"] = value.CreateFunction("new", []string{"status", "headers"}, func(args []interface{}) (interface{}, error) {
		init := value.CreateObject()
		status := 200
		if len(args) > 0 {
			switch v := args[0].(type) {
			case int:
				status = v
			case value.VsiNumber:
				status = v.Value
			case *value.VsiNumber:
				status = v.Value
			}
		}
		init.Proto["status"] = status
		if len(args) > 1 {
			if headers, ok := args[1].(*value.VsiObject); ok {
				init.Proto["headers"] = headers
			}
		}
		return init, nil
	})
	httpObj.Proto["ResponseInit"] = responseInitObj
	process.Proto["net"] = value.CreateObject()
	process.Proto["net"].(*value.VsiObject).Proto["http"] = httpObj

	// 冻结 process 对象
	process.Freeze()
	global.Variables["process"] = value.CreateVariable(process)

	// ==================== JSON 对象 ====================
	jsonObj := value.CreateObject()
	jsonObj.Proto["stringify"] = value.CreateFunction("stringify", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return "", nil
		}
		native := VsiToNative(args[0])
		b, err := json.Marshal(native)
		if err != nil {
			return "", err
		}
		return string(b), nil
	})
	jsonObj.Proto["parse"] = value.CreateFunction("parse", []string{"text"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
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
		var parsed interface{}
		if err := json.Unmarshal([]byte(s), &parsed); err != nil {
			return nil, err
		}
		return NativeToVsi(parsed), nil
	})
	global.Variables["JSON"] = value.CreateVariable(jsonObj)

	// ==================== String 构造函数和对象 ====================
	stringObj := createStringGlobal()
	global.Variables["String"] = value.CreateVariable(stringObj)

	// ==================== Number 构造函数 ====================
	numberObj := value.CreateObject()
	numberObj.Proto["parseInt"] = value.CreateFunction("parseInt", []string{"str"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return 0, nil
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
		var result int
		for _, ch := range s {
			if ch >= '0' && ch <= '9' {
				result = result*10 + int(ch-'0')
			} else {
				break
			}
		}
		return value.VsiNumber{Value: result}, nil
	})
	numberObj.Proto["parseFloat"] = value.CreateFunction("parseFloat", []string{"str"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return 0.0, nil
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
		var result float64
		var decimal float64 = 1
		var foundDot bool = false
		for _, ch := range s {
			if ch >= '0' && ch <= '9' {
				if foundDot {
					decimal *= 10
					result += float64(ch-'0') / decimal
				} else {
					result = result*10 + float64(ch-'0')
				}
			} else if ch == '.' && !foundDot {
				foundDot = true
			} else {
				break
			}
		}
		return result, nil
	})
	global.Variables["Number"] = value.CreateVariable(numberObj)

	// ==================== Boolean 构造函数 ====================
	boolObj := value.CreateObject()
	boolObj.Proto["parse"] = value.CreateFunction("parse", []string{"str"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
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
		return s == "true" || s == "1", nil
	})
	global.Variables["Boolean"] = value.CreateVariable(boolObj)

	// ==================== Array 构造函数 ====================
	arrayObj := value.CreateObject()
	arrayObj.Proto["isArray"] = value.CreateFunction("isArray", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}
		switch args[0].(type) {
		case *value.VsiArray:
			return true, nil
		default:
			return false, nil
		}
	})
	arrayObj.Proto["from"] = value.CreateFunction("from", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.CreateArray([]interface{}{}), nil
		}
		switch v := args[0].(type) {
		case *value.VsiArray:
			return v, nil
		case *value.VsiObject:
			items := []interface{}{}
			for k, val := range v.Proto {
				pair := value.CreateObject()
				pair.Proto["key"] = k
				pair.Proto["value"] = val
				items = append(items, pair)
			}
			return value.CreateArray(items), nil
		case string:
			items := []interface{}{}
			for _, ch := range v {
				items = append(items, value.VsiString{Value: string(ch)})
			}
			return value.CreateArray(items), nil
		case value.VsiString:
			items := []interface{}{}
			for _, ch := range v.Value {
				items = append(items, value.VsiString{Value: string(ch)})
			}
			return value.CreateArray(items), nil
		case *value.VsiString:
			items := []interface{}{}
			for _, ch := range v.Value {
				items = append(items, value.VsiString{Value: string(ch)})
			}
			return value.CreateArray(items), nil
		default:
			return value.CreateArray([]interface{}{}), nil
		}
	})
	global.Variables["Array"] = value.CreateVariable(arrayObj)

	// ==================== Object 构造函数 ====================
	objectObj := value.CreateObject()
	objectObj.Proto["keys"] = value.CreateFunction("keys", []string{"obj"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.CreateArray([]interface{}{}), nil
		}
		if obj, ok := args[0].(*value.VsiObject); ok {
			keys := []interface{}{}
			for k := range obj.Proto {
				keys = append(keys, value.VsiString{Value: k})
			}
			return value.CreateArray(keys), nil
		}
		return value.CreateArray([]interface{}{}), nil
	})
	objectObj.Proto["values"] = value.CreateFunction("values", []string{"obj"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.CreateArray([]interface{}{}), nil
		}
		if obj, ok := args[0].(*value.VsiObject); ok {
			values := []interface{}{}
			for _, v := range obj.Proto {
				values = append(values, v)
			}
			return value.CreateArray(values), nil
		}
		return value.CreateArray([]interface{}{}), nil
	})
	objectObj.Proto["entries"] = value.CreateFunction("entries", []string{"obj"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.CreateArray([]interface{}{}), nil
		}
		if obj, ok := args[0].(*value.VsiObject); ok {
			entries := []interface{}{}
			for k, v := range obj.Proto {
				entry := value.CreateArray([]interface{}{value.VsiString{Value: k}, v})
				entries = append(entries, entry)
			}
			return value.CreateArray(entries), nil
		}
		return value.CreateArray([]interface{}{}), nil
	})
	objectObj.Proto["assign"] = value.CreateFunction("assign", []string{"target", "source"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("assign requires target and source")
		}
		target, ok := args[0].(*value.VsiObject)
		if !ok {
			return nil, fmt.Errorf("target must be an object")
		}
		if target.Const {
			return nil, fmt.Errorf("cannot assign to frozen object")
		}
		source, ok := args[1].(*value.VsiObject)
		if !ok {
			return target, nil
		}
		for k, v := range source.Proto {
			target.Proto[k] = v
		}
		return target, nil
	})
	// Object.freeze - 将对象设为不可变
	objectObj.Proto["freeze"] = value.CreateFunction("freeze", []string{"obj"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}
		switch v := args[0].(type) {
		case *value.VsiObject:
			v.Freeze()
			return v, nil
		case *value.VsiArray:
			v.Freeze()
			return v, nil
		default:
			return args[0], nil
		}
	})
	// Object.isFrozen - 检查对象是否不可变
	objectObj.Proto["isFrozen"] = value.CreateFunction("isFrozen", []string{"obj"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}
		switch v := args[0].(type) {
		case *value.VsiObject:
			return v.IsFrozen(), nil
		case *value.VsiArray:
			return v.IsFrozen(), nil
		default:
			return false, nil
		}
	})
	// 冻结 Object 本身
	objectObj.Freeze()
	global.Variables["Object"] = value.CreateVariable(objectObj)

	// ==================== Error 构造函数 ====================
	errorObj := createErrorGlobal()
	global.Variables["Error"] = value.CreateVariable(errorObj)

	// ==================== 基础类型对象 ====================
	// int 类型
	intType := value.CreateObject()
	intType.Proto["name"] = "int"
	intType.Proto["check"] = value.CreateFunction("check", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}
		switch args[0].(type) {
		case int, value.VsiNumber, *value.VsiNumber:
			return true, nil
		default:
			return false, nil
		}
	})
	intType.Freeze()
	global.Variables["int"] = value.CreateVariable(intType)

	// string 类型
	stringType := value.CreateObject()
	stringType.Proto["name"] = "string"
	stringType.Proto["check"] = value.CreateFunction("check", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}
		switch args[0].(type) {
		case string, value.VsiString, *value.VsiString:
			return true, nil
		default:
			return false, nil
		}
	})
	stringType.Freeze()
	global.Variables["string"] = value.CreateVariable(stringType)

	// bool 类型
	boolType := value.CreateObject()
	boolType.Proto["name"] = "bool"
	boolType.Proto["check"] = value.CreateFunction("check", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}
		switch args[0].(type) {
		case bool:
			return true, nil
		default:
			return false, nil
		}
	})
	boolType.Freeze()
	global.Variables["bool"] = value.CreateVariable(boolType)

	// float 类型
	floatType := value.CreateObject()
	floatType.Proto["name"] = "float"
	floatType.Proto["check"] = value.CreateFunction("check", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}
		switch args[0].(type) {
		case int, float64, value.VsiNumber, *value.VsiNumber:
			return true, nil
		default:
			return false, nil
		}
	})
	floatType.Freeze()
	global.Variables["float"] = value.CreateVariable(floatType)

	// void 类型
	voidType := value.CreateObject()
	voidType.Proto["name"] = "void"
	voidType.Proto["check"] = value.CreateFunction("check", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 || args[0] == nil {
			return true, nil
		}
		return false, nil
	})
	voidType.Freeze()
	global.Variables["void"] = value.CreateVariable(voidType)

	// ==================== fetch 函数 ====================
	fetchFn := value.CreateFunction("fetch", []string{"url", "options"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("fetch requires a URL")
		}

		// 获取 URL
		var urlStr string
		switch v := args[0].(type) {
		case string:
			urlStr = v
		case value.VsiString:
			urlStr = v.Value
		case *value.VsiString:
			urlStr = v.Value
		default:
			urlStr = fmt.Sprint(v)
		}

		// 解析 options
		method := "GET"
		var body io.Reader
		headers := make(map[string]string)

		if len(args) > 1 {
			if opts, ok := args[1].(*value.VsiObject); ok {
				if m, ok := opts.Proto["method"]; ok {
					switch v := m.(type) {
					case string:
						method = strings.ToUpper(v)
					case value.VsiString:
						method = strings.ToUpper(v.Value)
					case *value.VsiString:
						method = strings.ToUpper(v.Value)
					}
				}
				if b, ok := opts.Proto["body"]; ok {
					switch v := b.(type) {
					case string:
						body = strings.NewReader(v)
					case value.VsiString:
						body = strings.NewReader(v.Value)
					case *value.VsiString:
						body = strings.NewReader(v.Value)
					}
				}
				if h, ok := opts.Proto["headers"]; ok {
					if headersObj, ok := h.(*value.VsiObject); ok {
						for k, v := range headersObj.Proto {
							switch val := v.(type) {
							case string:
								headers[k] = val
							case value.VsiString:
								headers[k] = val.Value
							case *value.VsiString:
								headers[k] = val.Value
							}
						}
					}
				}
			}
		}

		// 创建 HTTP 请求
		req, err := http.NewRequest(method, urlStr, body)
		if err != nil {
			return nil, err
		}

		// 设置 headers
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		// 发送请求
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// 读取响应体
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// 创建 Response 对象
		response := value.CreateObject()
		response.Proto["status"] = resp.StatusCode
		response.Proto["statusText"] = resp.Status
		response.Proto["ok"] = resp.StatusCode >= 200 && resp.StatusCode < 300
		response.Proto["url"] = urlStr

		// 响应头
		respHeaders := value.CreateObject()
		for k, v := range resp.Header {
			if len(v) > 0 {
				respHeaders.Proto[k] = v[0]
			}
		}
		response.Proto["headers"] = respHeaders

		// 原始响应体
		response.Proto["_body"] = string(respBody)

		// text() 方法
		response.Proto["text"] = value.CreateFunction("text", []string{}, func(args []interface{}) (interface{}, error) {
			if body, ok := response.Proto["_body"]; ok {
				if s, ok := body.(string); ok {
					return value.VsiString{Value: s}, nil
				}
			}
			return value.VsiString{Value: ""}, nil
		})

		// json() 方法
		response.Proto["json"] = value.CreateFunction("json", []string{}, func(args []interface{}) (interface{}, error) {
			if body, ok := response.Proto["_body"]; ok {
				if s, ok := body.(string); ok {
					var parsed interface{}
					if err := json.Unmarshal([]byte(s), &parsed); err != nil {
						return nil, err
					}
					return NativeToVsi(parsed), nil
				}
			}
			return nil, nil
		})

		return response, nil
	})
	global.Variables["fetch"] = value.CreateVariable(fetchFn)

	return global
}

// createStringGlobal 创建 String 全局对象
func createStringGlobal() *value.VsiObject {
	stringObj := value.CreateObject()

	// String() 构造函数
	// 1. 传入数字数组，返回连续文本
	// 2. 传入数字 unicode，返回对应字符
	// 3. 传入字符串，返回字符串本身
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

	// 从数字数组创建字符串
	stringObj.Proto["fromArray"] = value.CreateFunction("fromArray", []string{"arr"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.VsiString{Value: ""}, nil
		}
		result := ""
		switch v := args[0].(type) {
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
		return value.VsiString{Value: result}, nil
	})

	// 字符转 Unicode 编码
	stringObj.Proto["toCharCode"] = value.CreateFunction("toCharCode", []string{"str"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.VsiNumber{Value: 0}, nil
		}
		var s string
		switch v := args[0].(type) {
		case string:
			s = v
		case value.VsiString:
			s = v.Value
		case *value.VsiString:
			s = v.Value
		case int:
			return value.VsiNumber{Value: v}, nil
		case value.VsiNumber:
			return v, nil
		case *value.VsiNumber:
			return *v, nil
		default:
			s = fmt.Sprint(v)
		}
		if len(s) == 0 {
			return value.VsiNumber{Value: 0}, nil
		}
		return value.VsiNumber{Value: int([]rune(s)[0])}, nil
	})

	// StringToUnicodeNumber - 字符串转 Unicode 数字数组
	stringObj.Proto["toUnicodeArray"] = value.CreateFunction("toUnicodeArray", []string{"str"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
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
		runes := []rune(s)
		codes := make([]interface{}, len(runes))
		for i, r := range runes {
			codes[i] = value.VsiNumber{Value: int(r)}
		}
		return value.CreateArray(codes), nil
	})

	// 创建 VsiString 原型
	stringProto := createVsiStringPrototype()
	stringObj.Proto["prototype"] = stringProto

	return stringObj
}

// createVsiStringPrototype 创建 VsiString 原型方法
func createVsiStringPrototype() *value.VsiObject {
	proto := value.CreateObject()

	// slice 切片
	proto.Proto["slice"] = value.CreateFunction("slice", []string{"start", "end"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.VsiString{Value: ""}, nil
		}
		// this should be the string itself, but in current implementation
		// we need to pass the string as first arg
		return value.VsiString{Value: ""}, fmt.Errorf("slice requires a string context")
	})

	// split 分割
	proto.Proto["split"] = value.CreateFunction("split", []string{"separator"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return value.CreateArray([]interface{}{}), fmt.Errorf("split requires string context and separator")
		}
		var str string
		switch v := args[0].(type) {
		case string:
			str = v
		case value.VsiString:
			str = v.Value
		case *value.VsiString:
			str = v.Value
		default:
			str = fmt.Sprint(v)
		}
		var sep string
		switch v := args[1].(type) {
		case string:
			sep = v
		case value.VsiString:
			sep = v.Value
		case *value.VsiString:
			sep = v.Value
		default:
			sep = fmt.Sprint(v)
		}
		parts := splitString(str, sep)
		items := make([]interface{}, len(parts))
		for i, p := range parts {
			items[i] = value.VsiString{Value: p}
		}
		return value.CreateArray(items), nil
	})

	// length 长度
	proto.Proto["length"] = value.CreateFunction("length", []string{}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.VsiNumber{Value: 0}, nil
		}
		var str string
		switch v := args[0].(type) {
		case string:
			str = v
		case value.VsiString:
			str = v.Value
		case *value.VsiString:
			str = v.Value
		default:
			str = fmt.Sprint(v)
		}
		return value.VsiNumber{Value: len([]rune(str))}, nil
	})

	// charAt 获取指定位置字符
	proto.Proto["charAt"] = value.CreateFunction("charAt", []string{"index"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return value.VsiString{Value: ""}, nil
		}
		var str string
		switch v := args[0].(type) {
		case string:
			str = v
		case value.VsiString:
			str = v.Value
		case *value.VsiString:
			str = v.Value
		default:
			str = fmt.Sprint(v)
		}
		var index int
		switch v := args[1].(type) {
		case int:
			index = v
		case value.VsiNumber:
			index = v.Value
		case *value.VsiNumber:
			index = v.Value
		}
		runes := []rune(str)
		if index < 0 || index >= len(runes) {
			return value.VsiString{Value: ""}, nil
		}
		return value.VsiString{Value: string(runes[index])}, nil
	})

	// charCodeAt 获取指定位置字符的 Unicode 编码
	proto.Proto["charCodeAt"] = value.CreateFunction("charCodeAt", []string{"index"}, func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return value.VsiNumber{Value: -1}, nil
		}
		var str string
		switch v := args[0].(type) {
		case string:
			str = v
		case value.VsiString:
			str = v.Value
		case *value.VsiString:
			str = v.Value
		default:
			str = fmt.Sprint(v)
		}
		var index int
		switch v := args[1].(type) {
		case int:
			index = v
		case value.VsiNumber:
			index = v.Value
		case *value.VsiNumber:
			index = v.Value
		}
		runes := []rune(str)
		if index < 0 || index >= len(runes) {
			return value.VsiNumber{Value: -1}, nil
		}
		return value.VsiNumber{Value: int(runes[index])}, nil
	})

	// toUpperCase 转大写
	proto.Proto["toUpperCase"] = value.CreateFunction("toUpperCase", []string{}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.VsiString{Value: ""}, nil
		}
		var str string
		switch v := args[0].(type) {
		case string:
			str = v
		case value.VsiString:
			str = v.Value
		case *value.VsiString:
			str = v.Value
		default:
			str = fmt.Sprint(v)
		}
		result := ""
		for _, ch := range str {
			if ch >= 'a' && ch <= 'z' {
				result += string(ch - 32)
			} else {
				result += string(ch)
			}
		}
		return value.VsiString{Value: result}, nil
	})

	// toLowerCase 转小写
	proto.Proto["toLowerCase"] = value.CreateFunction("toLowerCase", []string{}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.VsiString{Value: ""}, nil
		}
		var str string
		switch v := args[0].(type) {
		case string:
			str = v
		case value.VsiString:
			str = v.Value
		case *value.VsiString:
			str = v.Value
		default:
			str = fmt.Sprint(v)
		}
		result := ""
		for _, ch := range str {
			if ch >= 'A' && ch <= 'Z' {
				result += string(ch + 32)
			} else {
				result += string(ch)
			}
		}
		return value.VsiString{Value: result}, nil
	})

	// trim 去除首尾空白
	proto.Proto["trim"] = value.CreateFunction("trim", []string{}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return value.VsiString{Value: ""}, nil
		}
		var str string
		switch v := args[0].(type) {
		case string:
			str = v
		case value.VsiString:
			str = v.Value
		case *value.VsiString:
			str = v.Value
		default:
			str = fmt.Sprint(v)
		}
		// simple trim
		start := 0
		end := len(str) - 1
		for start <= end && (str[start] == ' ' || str[start] == '\t' || str[start] == '\n' || str[start] == '\r') {
			start++
		}
		for end >= start && (str[end] == ' ' || str[end] == '\t' || str[end] == '\n' || str[end] == '\r') {
			end--
		}
		return value.VsiString{Value: str[start : end+1]}, nil
	})

	return proto
}

// createErrorGlobal 创建 Error 全局对象
func createErrorGlobal() *value.VsiObject {
	errorObj := value.CreateObject()

	// Error(message) 构造函数
	errorObj.Proto["new"] = value.CreateFunction("new", []string{"message"}, func(args []interface{}) (interface{}, error) {
		message := ""
		if len(args) > 0 {
			switch v := args[0].(type) {
			case string:
				message = v
			case value.VsiString:
				message = v.Value
			case *value.VsiString:
				message = v.Value
			default:
				message = fmt.Sprint(v)
			}
		}
		return &types.VsiError{
			Message:   message,
			ErrorType: "Error",
			Stack:     []types.StackFrame{},
		}, nil
	})

	// 错误类型
	errorTypes := []string{"TypeError", "RangeError", "SyntaxError", "RuntimeError", "ReferenceError"}
	for _, errType := range errorTypes {
		errObj := value.CreateObject()
		errTypeCopy := errType // capture
		errObj.Proto["new"] = value.CreateFunction("new", []string{"message"}, func(args []interface{}) (interface{}, error) {
			message := ""
			if len(args) > 0 {
				switch v := args[0].(type) {
				case string:
					message = v
				case value.VsiString:
					message = v.Value
				case *value.VsiString:
					message = v.Value
				default:
					message = fmt.Sprint(v)
				}
			}
			return &types.VsiError{
				Message:   message,
				ErrorType: errTypeCopy,
				Stack:     []types.StackFrame{},
			}, nil
		})
		errorObj.Proto[errType] = errObj
	}

	return errorObj
}

// splitString 分割字符串（简单实现）
func splitString(str, sep string) []string {
	if sep == "" {
		result := make([]string, len(str))
		for i, ch := range str {
			result[i] = string(ch)
		}
		return result
	}
	var result []string
	for {
		idx := findSubstring(str, sep)
		if idx == -1 {
			result = append(result, str)
			break
		}
		result = append(result, str[:idx])
		str = str[idx+len(sep):]
	}
	return result
}

// findSubstring 查找子字符串位置
func findSubstring(str, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ==================== 程序上下文创建 ====================

// CreateProgramContext 创建程序上下文
func CreateProgramContext(filename string) *ProgramContext {
	global := createGlobalContext()
	file := NewContext(LevelFile, global)

	return &ProgramContext{
		Global:      global,
		File:        file,
		Current:     file,
		Stack:       []types.StackFrame{},
		CurrentFile: filename,
	}
}

// PushStack 压入调用栈
func (pc *ProgramContext) PushStack(frame types.StackFrame) {
	pc.Stack = append(pc.Stack, frame)
}

// PopStack 弹出调用栈
func (pc *ProgramContext) PopStack() {
	if len(pc.Stack) > 0 {
		pc.Stack = pc.Stack[:len(pc.Stack)-1]
	}
}

// FormatStack 格式化调用栈
func (pc *ProgramContext) FormatStack() string {
	result := ""
	for i := len(pc.Stack) - 1; i >= 0; i-- {
		result += "  " + pc.Stack[i].String() + "\n"
	}
	return result
}

// LookupVariable 在上下文中查找变量
func (pc *ProgramContext) LookupVariable(name string) (*value.VsiVariable, bool) {
	return pc.Current.LookupVariable(name)
}

// LookupFunction 在上下文中查找函数
func (pc *ProgramContext) LookupFunction(name string) (*value.VsiFunction, bool) {
	return pc.Current.LookupFunction(name)
}

// LookupClass 在上下文中查找类
func (pc *ProgramContext) LookupClass(name string) (*types.ClassDeclaration, bool) {
	return pc.Current.LookupClass(name)
}

// SetVariable 设置变量
func (pc *ProgramContext) SetVariable(name string, v *value.VsiVariable, isTop bool) {
	if isTop {
		pc.File.Variables[name] = v
	} else {
		pc.Current.Variables[name] = v
	}
}

// SetFunction 设置函数
func (pc *ProgramContext) SetFunction(name string, f *value.VsiFunction, isTop bool) {
	if isTop {
		pc.File.Functions[name] = f
	} else {
		pc.Current.Functions[name] = f
	}
}

// SetClass 设置类
func (pc *ProgramContext) SetClass(name string, c *types.ClassDeclaration, isTop bool) {
	if isTop {
		pc.File.Classes[name] = c
	} else {
		pc.Current.Classes[name] = c
	}
}

// CreateLocalContext 创建局部作用域
func (pc *ProgramContext) CreateLocalContext() *Context {
	return NewContext(LevelLocal, pc.Current)
}

// EnterContext 进入新上下文
func (pc *ProgramContext) EnterContext(ctx *Context) {
	pc.Current = ctx
}

// ExitContext 退出当前上下文
func (pc *ProgramContext) ExitContext() {
	if pc.Current.Parent != nil {
		pc.Current = pc.Current.Parent
	}
}
