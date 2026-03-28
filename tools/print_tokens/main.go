package main

import (
	"fmt"

	"github.com/RuanhoR/vsi/internal/compiler/tokenizr"
)

func main() {
	code := `var s = "{\"a\":1,\"b\":[1,2,3]}"; var o = JSON.parse(s); process.stdout.write(JSON.stringify(o));`
	tokens := tokenizr.GenerateTokenizr(code)
	for i, t := range tokens {
		fmt.Printf("%02d: Type=%s Data=%q\n", i, t.Type, t.Data)
	}
}
