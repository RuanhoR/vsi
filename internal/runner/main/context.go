package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/RuanhoR/vsi/internal/runner/value"
	"github.com/RuanhoR/vsi/internal/types"
	"github.com/RuanhoR/vsi/pkg/config"
)

func createContext() *types.ProgramContext {
	c := &types.ProgramContext{
		Top: &types.Context{
			Variables: make(map[string]*value.VsiVariable),
			Functions: make(map[string]*value.VsiFunction),
			Imports:   make(map[string]interface{}),
		},
		Current: &types.Context{
			Variables: make(map[string]*value.VsiVariable),
			Functions: make(map[string]*value.VsiFunction),
			Imports:   make(map[string]interface{}),
		},
	}

	process := value.CreateObject()
	// env: map[string]string of environment
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		// split at first =
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}
	process.Proto["env"] = envMap
	process.Proto["argv"] = os.Args
	process__version := value.CreateObject()
	process__version.Proto["vsi"] = config.Version
	process__version.Proto["go"] = runtime.Version()
	process.Proto["version"] = process__version
	process.Proto["pid"] = os.Getpid()
	process.Proto["platform"] = value.VsiString{Value: runtime.GOOS}
	process.Proto["arch"] = value.VsiString{Value: runtime.GOARCH}
	home, _ := os.Getwd()
	process.Proto["cwd"] = value.VsiString{Value: home}
	// add stdout with write function
	stdout := value.CreateObject()
	stdout.Proto["write"] = value.CreateFunction("write", []string{"data"}, func(args []interface{}) (interface{}, error) {
		if len(args) > 0 {
			// print first arg to stdout
			switch v := args[0].(type) {
			case string:
				os.Stdout.WriteString(v)
			case int:
				os.Stdout.WriteString(fmt.Sprint(v))
			default:
				os.Stdout.WriteString(fmt.Sprint(v))
			}
		}
		return nil, nil
	})
	fileObj := value.CreateObject()
	fileObj.Proto["readFile"] = value.CreateFunction("ReadFile", []string{}, func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("readFile requires at least one argument")
		}
		result, err := os.ReadFile(args[0].(string))
		if err != nil {
			return nil, err
		}
		resultArr := []int{}
		for _, data := range result {
			resultArr = append(resultArr, int(data))
		}
		// convert []int to []interface{}
		resultIface := make([]interface{}, len(resultArr))
		for i, v := range resultArr {
			resultIface[i] = v
		}
		return value.CreateArray(resultIface), nil
	})
	process.Proto["file"] = fileObj
	process.Proto["stdout"] = stdout
	// add path utilities under process.path
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
	// add global JSON object with stringify and parse
	jsonObj := value.CreateObject()
	jsonObj.Proto["stringify"] = value.CreateFunction("stringify", []string{"value"}, func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return "", nil
		}
		native := vsiToNative(args[0])
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
			// fallback to fmt
			s = fmt.Sprint(v)
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(s), &parsed); err != nil {
			return nil, err
		}
		return nativeToVsi(parsed), nil
	})
	// expose JSON as global object
	c.Top.Variables["JSON"] = value.CreateVariable(jsonObj)
	consoleObj := value.CreateObject()
	consoleObj.Proto["log"] = value.CreateFunction("log", []string{"data"}, func(args []interface{}) (interface{}, error) {
		if len(args) > 0 {
			// print first arg to stdout
			switch v := args[0].(type) {
			case string:
				fmt.Println(v)
			case int:
				fmt.Println(fmt.Sprint(v))
			default:
				fmt.Println(fmt.Sprint(v))
			}
		}
		return nil, nil
	})
	process.Proto["console"] = consoleObj
	c.Top.Variables["process"] = value.CreateVariable(process)
	return c
}
