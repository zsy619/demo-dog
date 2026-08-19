// Package mustx 提供 panic on error 风格的辅助。
package mustx

import "fmt"

// NoError 在 err != nil 时 panic。
func NoError(err error) {
	if err != nil {
		panic(err)
	}
}

// NoErrorFn 调用 fn，若返回错误则 panic。
func NoErrorFn[T any](fn func() (T, error)) T {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

// True 在 cond 为 false 时 panic。
func True(cond bool, msg string) {
	if !cond {
		panic(fmt.Sprintf("mustx: %s", msg))
	}
}

// NotNil 在 v 为 nil 时 panic。
func NotNil(v any, msg string) {
	if v == nil {
		panic(fmt.Sprintf("mustx: %s", msg))
	}
}
