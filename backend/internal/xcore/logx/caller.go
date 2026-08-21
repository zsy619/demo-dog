package logx

// caller.go:Caller 与 trimPath。

import (
	"fmt"
	"runtime"
)

// Caller 返回直接调用方的函数名与行号(在本包内 skip=1)。
func Caller(skip int) Field {
	pc, _, _, ok := runtime.Caller(skip + 1)
	if !ok {
		return Str("caller", "unknown")
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return Str("caller", "unknown")
	}
	file, line := fn.FileLine(pc)
	return Str("caller", fmt.Sprintf("%s:%d", trimPath(file), line))
}

// trimPath 去掉路径前缀,只保留文件名。
func trimPath(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
