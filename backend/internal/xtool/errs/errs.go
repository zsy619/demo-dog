// Package errs 提供一个结构化错误码 + 消息的错误类型。
package errs

import (
	"errors"
	"fmt"
)

// Code 是错误码。
type Code int

// E 是结构化错误。
type E struct {
	Code    Code
	Message string
	Cause   error
}

// New 创建一个新错误。
func New(code Code, msg string) *E {
	return &E{Code: code, Message: msg}
}

// Wrap 把底层错误包装为结构化错误。
func Wrap(code Code, cause error, msg string) *E {
	return &E{Code: code, Message: msg, Cause: cause}
}

// Error 实现 error 接口。
func (e *E) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap。
func (e *E) Unwrap() error { return e.Cause }

// CodeOf 返回错误码（如果不是 E 返回 0）。
func CodeOf(err error) Code {
	var e *E
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}

// MessageOf 返回错误消息。
func MessageOf(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
