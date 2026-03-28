package repl

import (
	"os"

	"golang.org/x/term"
)

var shouldStop = false

func startInputListener(onInput func(string)) {
	// 保存原始终端设置
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// 如果不是终端，直接返回
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// 使用更大的缓冲区读取UTF-8字符
	buf := make([]byte, 1024)

	for !shouldStop {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		if n > 0 {
			// 将字节转为字符串（支持UTF-8）
			input := string(buf[:n])
			onInput(input)
		}
	}
}

// 停止监听
func stopInputListener() {
	shouldStop = true
}
