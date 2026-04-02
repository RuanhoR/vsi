package vsic

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/RuanhoR/vsi/internal/runner/value"
)

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

// SetupGlobals 设置内置全局变量和函数
func (vm *VM) SetupGlobals() {
	// 设置内置函数
	vm.globals["print"] = value.CreateFunction("print", []string{"msg"}, func(args []interface{}) (interface{}, error) {
		if len(args) > 0 {
			fmt.Fprintf(vm.stdout, "%v", args[0])
		}
		fmt.Fprintf(vm.stdout, "\n")
		return nil, nil
	})

	vm.globals["input"] = value.CreateFunction("input", []string{"prompt"}, func(args []interface{}) (interface{}, error) {
		if len(args) > 0 {
			fmt.Fprintf(vm.stdout, "%v", args[0])
		}
		var input string
		fmt.Fscanf(os.Stdin, "%s", &input)
		return input, nil
	})

	vm.globals["len"] = value.CreateFunction("len", []string{"obj"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return 0, nil
		}

		switch v := args[0].(type) {
		case string:
			return len(v), nil
		case []interface{}:
			return len(v), nil
		case *value.VsiArray:
			return len(v.Items), nil
		default:
			return 0, nil
		}
	})

	// 设置内置常量
	vm.globals["true"] = true
	vm.globals["false"] = false
	vm.globals["nil"] = nil

	// 设置内置全局对象
	vm.setupStringObject()
	vm.setupProcessObject()
}

// setupStringObject 设置 String 构造函数和方法
func (vm *VM) setupStringObject() {
	// 创建 String 构造函数对象
	stringConstructor := map[string]interface{}{
		"new": value.CreateFunction("String.new", []string{"value"}, func(args []interface{}) (interface{}, error) {
			// 调试：输出参数信息
			if os.Getenv("VSIC_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG String.new: len(args)=%d\n", len(args))
				for i, arg := range args {
					fmt.Fprintf(os.Stderr, "  args[%d]: type=%T, value=%v\n", i, arg, arg)
				}
			}

			// 在方法调用中，args[0] 可能是 this 对象，args[1] 才是真正的参数
			var actualArg interface{}
			if len(args) >= 2 {
				actualArg = args[1] // 方法调用：this + 参数
			} else if len(args) == 1 {
				actualArg = args[0] // 构造函数调用：只有参数
			} else {
				return "", nil
			}

			// 处理不同类型的参数
			switch v := actualArg.(type) {
			case string:
				return v, nil
			case int:
				// unicode 对应字符
				return string(rune(v)), nil
			case *value.VsiNumber:
				// unicode 对应字符
				return string(rune(v.Value)), nil
			case []interface{}:
				// 处理数组：VsiNumber[] 或 int[] => unicode
				result := ""
				for _, item := range v {
					switch itemVal := item.(type) {
					case int:
						result += string(rune(itemVal))
					case *value.VsiNumber:
						result += string(rune(itemVal.Value))
					default:
						// 其他类型转换为字符串
						result += fmt.Sprintf("%v", itemVal)
					}
				}
				return result, nil
			default:
				// 其他类型直接转换为字符串
				return fmt.Sprintf("%v", v), nil
			}
		}),

		"split": value.CreateFunction("String.split", []string{"str", "separator"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return []interface{}{}, nil
			}

			strVal := ""
			separator := ","

			if args[0] != nil {
				strVal = fmt.Sprintf("%v", args[0])
			}

			if len(args) > 1 && args[1] != nil {
				separator = fmt.Sprintf("%v", args[1])
			}

			parts := strings.Split(strVal, separator)
			result := make([]interface{}, len(parts))
			for i, part := range parts {
				result[i] = part
			}
			return result, nil
		}),

		"slice": value.CreateFunction("String.slice", []string{"str", "start", "end"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return "", nil
			}

			strVal := ""
			if args[0] != nil {
				strVal = fmt.Sprintf("%v", args[0])
			}

			runes := []rune(strVal)
			start := 0
			end := len(runes)

			if len(args) > 1 && args[1] != nil {
				if num, ok := args[1].(int); ok {
					start = num
				} else if num, ok := args[1].(*value.VsiNumber); ok {
					start = int(num.Value)
				}
			}

			if len(args) > 2 && args[2] != nil {
				if num, ok := args[2].(int); ok {
					end = num
				} else if num, ok := args[2].(*value.VsiNumber); ok {
					end = int(num.Value)
				}
			}

			// 处理负索引
			if start < 0 {
				start = len(runes) + start
				if start < 0 {
					start = 0
				}
			}

			if end < 0 {
				end = len(runes) + end
				if end < 0 {
					end = 0
				}
			}

			if start > len(runes) {
				start = len(runes)
			}

			if end > len(runes) {
				end = len(runes)
			}

			if start > end {
				return "", nil
			}

			result := string(runes[start:end])
			return result, nil
		}),
	}

	vm.globals["String"] = stringConstructor
}

// setupProcessObject 设置 process 对象和其属性和方法
func (vm *VM) setupProcessObject() {
	// 创建 process 对象
	process := map[string]interface{}{}

	// process.cwd - 当前工作目录
	cwd, _ := os.Getwd()
	process["cwd"] = cwd

	// process.env - 环境变量
	process["env"] = map[string]interface{}{}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			process["env"].(map[string]interface{})[parts[0]] = parts[1]
		}
	}

	// process.stdout - 标准输出对象
	process["stdout"] = map[string]interface{}{
		"write": value.CreateFunction("stdout.write", []string{"data"}, func(args []interface{}) (interface{}, error) {
			if len(args) > 0 {
				fmt.Fprintf(vm.stdout, "%v", args[0])
			}
			return nil, nil
		}),
	}

	// process.path - 路径操作对象
	process["path"] = map[string]interface{}{
		"join": value.CreateFunction("path.join", []string{}, func(args []interface{}) (interface{}, error) {
			var paths []string
			for _, arg := range args {
				if arg != nil {
					paths = append(paths, fmt.Sprintf("%v", arg))
				}
			}

			// 使用 filepath.Join 处理路径
			if len(paths) == 0 {
				return "", nil
			}

			result := filepath.Join(paths...)
			return result, nil
		}),

		"dirname": value.CreateFunction("path.dirname", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return ".", nil
			}

			path := fmt.Sprintf("%v", args[0])
			return filepath.Dir(path), nil
		}),

		"basename": value.CreateFunction("path.basename", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return "", nil
			}

			path := fmt.Sprintf("%v", args[0])
			return filepath.Base(path), nil
		}),

		"extname": value.CreateFunction("path.extname", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return "", nil
			}

			path := fmt.Sprintf("%v", args[0])
			return filepath.Ext(path), nil
		}),

		"parse": value.CreateFunction("path.parse", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return map[string]interface{}{
					"root": "",
					"dir":  "",
					"base": "",
					"ext":  "",
					"name": "",
				}, nil
			}

			path := fmt.Sprintf("%v", args[0])

			result := map[string]interface{}{}
			result["root"] = filepath.VolumeName(path) + string(filepath.Separator)
			result["dir"] = filepath.Dir(path)
			result["base"] = filepath.Base(path)
			result["ext"] = filepath.Ext(path)

			// 获取不带扩展名的文件名
			base := filepath.Base(path)

			extPos := strings.LastIndex(base, ".")
			if extPos > 0 {
				result["name"] = base[:extPos]
			} else {
				result["name"] = base
			}

			return result, nil
		}),
	}

	// process.file - 文件操作对象
	process["file"] = map[string]interface{}{

		"readFile": value.CreateFunction("file.readFile", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return "", nil
			}

			filePath := fmt.Sprintf("%v", args[0])

			// 调试：输出当前工作目录和文件路径信息
			if os.Getenv("VSIC_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG readFile: original path=%s, cwd=%s\n", filePath, cwd)
			}

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			content, err := ioutil.ReadFile(filePath)
			if err != nil {
				return "", fmt.Errorf("无法读取文件: %v (resolved path: %s)", err, filePath)
			}

			// 调试：输出实际读取的文件内容
			if os.Getenv("VSIC_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "DEBUG readFile: successfully read %d bytes from %s\n", len(content), filePath)
			}

			return string(content), nil
		}),

		"readFileWithIndex": value.CreateFunction("file.readFileWithIndex", []string{"path", "start", "end"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return "", nil
			}

			filePath := fmt.Sprintf("%v", args[0])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			content, err := ioutil.ReadFile(filePath)
			if err != nil {
				return "", fmt.Errorf("无法读取文件: %v", err)
			}

			start := 0
			end := len(content)

			if len(args) > 1 && args[1] != nil {
				if num, ok := args[1].(int); ok {
					start = num
				} else if num, ok := args[1].(*value.VsiNumber); ok {
					start = int(num.Value)
				}
			}

			if len(args) > 2 && args[2] != nil {
				if num, ok := args[2].(int); ok {
					end = num
				} else if num, ok := args[2].(*value.VsiNumber); ok {
					end = int(num.Value)
				}
			}

			// 边界检查
			if start < 0 {
				start = 0
			}
			if end > len(content) {
				end = len(content)
			}
			if start > end {
				start = end
			}

			return string(content[start:end]), nil
		}),

		"fileExists": value.CreateFunction("file.fileExists", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return false, nil
			}

			filePath := fmt.Sprintf("%v", args[0])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			_, err := os.Stat(filePath)
			return err == nil, nil
		}),

		"writeFile": value.CreateFunction("file.writeFile", []string{"path", "data"}, func(args []interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("缺少路径或数据参数")
			}

			filePath := fmt.Sprintf("%v", args[0])
			data := fmt.Sprintf("%v", args[1])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			err := ioutil.WriteFile(filePath, []byte(data), 0644)
			return nil, err
		}),

		"mkdir": value.CreateFunction("file.mkdir", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("缺少路径参数")
			}

			filePath := fmt.Sprintf("%v", args[0])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			err := os.MkdirAll(filePath, 0755)
			return nil, err
		}),

		"rmdir": value.CreateFunction("file.rmdir", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("缺少路径参数")
			}

			filePath := fmt.Sprintf("%v", args[0])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			err := os.RemoveAll(filePath)
			return nil, err
		}),

		"rm": value.CreateFunction("file.rm", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("缺少路径参数")
			}

			filePath := fmt.Sprintf("%v", args[0])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			err := os.Remove(filePath)
			return nil, err
		}),

		"cp": value.CreateFunction("file.cp", []string{"src", "dst"}, func(args []interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("缺少源路径或目标路径参数")
			}

			srcPath := fmt.Sprintf("%v", args[0])
			dstPath := fmt.Sprintf("%v", args[1])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(srcPath) {
				srcPath = filepath.Join(cwd, srcPath)
			}
			if !filepath.IsAbs(dstPath) {
				dstPath = filepath.Join(cwd, dstPath)
			}

			content, err := ioutil.ReadFile(srcPath)
			if err != nil {
				return nil, err
			}

			err = ioutil.WriteFile(dstPath, content, 0644)
			return nil, err
		}),

		"createFile": value.CreateFunction("file.createFile", []string{"path"}, func(args []interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("缺少路径参数")
			}

			filePath := fmt.Sprintf("%v", args[0])

			// 如果路径是相对路径，基于当前工作目录
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cwd, filePath)
			}

			file, err := os.Create(filePath)
			if err != nil {
				return nil, err
			}
			file.Close()
			return nil, nil
		}),
	} // process.file 结束括号

	vm.globals["process"] = process
}
