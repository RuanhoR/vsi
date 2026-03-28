package tokenizr

import (
	"strconv"
	"strings"
	"unicode"
)

// TokenData 表示一个词法单元
type TokenData struct {
	Type string
	Data string
}

var keywordList = []string{
	"var",
	"const",
	"fun",
	"if",
	"for",
	"while",
	"import",
	"as",
	"export",
}

// 判断是否为数字
func IsNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// 查找下一个满足条件的位置
func findNext(arr []rune, start int, check func(rune) bool) int {
	if start < 0 {
		start = 0
	}
	if start >= len(arr) {
		return -1
	}
	for i := start; i < len(arr); i++ {
		if check(arr[i]) {
			return i
		}
	}
	return len(arr)
}

// 查找匹配的闭合引号
func findMatchingQuote(arr []rune, start int, delim rune) int {
	for i := start; i < len(arr); i++ {
		if arr[i] == delim {
			// count preceding backslashes to determine if this quote is escaped
			backslashes := 0
			j := i - 1
			for j >= 0 && arr[j] == '\\' {
				backslashes++
				j--
			}
			if backslashes%2 == 0 {
				return i
			}
			// else quote is escaped, continue searching
		}
	}
	return -1
}

// 判断 slice 中是否包含字符串
func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// 判断 slice 中是否包含 rune
func containsRune(list []rune, target rune) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// 获取符号类型
func getSymbolType(ch rune) string {
	switch ch {
	case '(':
		return "ParametersStart"
	case ')':
		return "ParametersEnd"
	case '{':
		return "BodyStart"
	case '}':
		return "BodyEnd"
	case '[':
		return "ArrayStart"
	case ']':
		return "ArrayEnd"
	case ';':
		return "SplitSymbol"
	case ':':
		return "KeyNext"
	case ',':
		return "SplitSymbol"
	case '.':
		return "MemberSymbol"
	default:
		return "Unknown"
	}
}

// 判断空白字符
func isSpace(ch rune) bool {
	return unicode.IsSpace(ch)
}

// 主分词函数
func GenerateTokenizr(code string) []TokenData {
	// support simple escaped newlines in input strings (e.g., "\n" passed via CLI)
	code = strings.ReplaceAll(code, "\\n", "\n")
	code = strings.ReplaceAll(code, "\\t", "\t")
	code = strings.ReplaceAll(code, "\\r", "\r")
	var tokens []TokenData
	runes := []rune(code)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// 跳过空白
		if isSpace(ch) {
			i++
			continue
		}
		// 注释
		if i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '/' {
			next := findNext(runes, i, func(r rune) bool {
				return r == '\n'
			})
			i = next + 1
			continue
		}
		// 字符串
		if ch == '"' || ch == '\'' {
			delim := ch
			start := i + 1
			end := findMatchingQuote(runes, start, delim)
			if end == -1 {
				end = len(runes)
			}
			tokens = append(tokens,
				TokenData{Type: "StringStart", Data: string(delim)},
				TokenData{Type: "StringContent", Data: string(runes[start:end])},
				TokenData{Type: "StringEnd", Data: string(delim)},
			)
			i = end + 1
			continue
		}

		// 单字符符号
		if containsRune([]rune("(){}[];:,."), ch) {
			tokens = append(tokens, TokenData{Type: getSymbolType(ch), Data: string(ch)})
			i++
			continue
		}

		// 运算符
		if strings.ContainsRune("+-*/", ch) {
			tokens = append(tokens, TokenData{Type: "Operator", Data: string(ch)})
			i++
			continue
		}

		// 赋值符
		if ch == '=' {
			tokens = append(tokens, TokenData{Type: "Assignment", Data: "="})
			i++
			continue
		}

		// 标识符 / 关键字
		if unicode.IsLetter(ch) || ch == '_' || ch == '$' {
			j := i
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_' || runes[j] == '$') {
				j++
			}
			word := string(runes[i:j])
			typ := "Identifier"
			if contains(keywordList, word) {
				typ = "Keyword"
			}
			tokens = append(tokens, TokenData{Type: typ, Data: word})
			i = j
			continue
		}

		// 数字
		if unicode.IsDigit(ch) {
			j := i
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
			tokens = append(tokens, TokenData{Type: "NumberLiteral", Data: string(runes[i:j])})
			i = j
			continue
		}

		// 未知字符跳过
		i++
	}

	return tokens
}
