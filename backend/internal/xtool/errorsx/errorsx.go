// Package errorsx 提供错误分类、聚合与上下文包装的辅助函数。
package errorsx

import (
	"errors"
	"strings"
)

// Kind 表示错误类别。
type Kind int

const (
	KindUnknown Kind = iota
	KindInvalid
	KindNotFound
	KindPermission
	KindTimeout
	KindCanceled
	KindUnavailable
	KindInternal
)

// String 返回类别名称。
func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindNotFound:
		return "not_found"
	case KindPermission:
		return "permission"
	case KindTimeout:
		return "timeout"
	case KindCanceled:
		return "canceled"
	case KindUnavailable:
		return "unavailable"
	case KindInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// E 是带类别的错误。
type E struct {
	Kind    Kind
	Code    string
	Message string
	Cause   error
}

// Error 实现 error 接口。
func (e *E) Error() string {
	if e.Cause != nil {
		return e.Kind.String() + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Kind.String() + ": " + e.Message
}

// Unwrap 返回底层错误。
func (e *E) Unwrap() error { return e.Cause }

// IsKind 判断错误是否为指定类别。
func IsKind(err error, k Kind) bool {
	var e *E
	if errors.As(err, &e) {
		return e.Kind == k
	}
	return false
}

// New 创建一个分类错误。
func New(k Kind, msg string) error { return &E{Kind: k, Message: msg} }

// Wrap 包装一个已有错误。
func Wrap(k Kind, err error, msg string) error {
	return &E{Kind: k, Message: msg, Cause: err}
}

// Code 返回错误的业务码。
func Code(err error) string {
	var e *E
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// KindOf 返回错误的类别。
func KindOf(err error) Kind {
	var e *E
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindUnknown
}

// Multi 合并多个错误。
type Multi struct {
	Errors []error
}

// Error 返回全部错误消息。
func (m *Multi) Error() string {
	parts := make([]string, 0, len(m.Errors))
	for _, e := range m.Errors {
		if e != nil {
			parts = append(parts, e.Error())
		}
	}
	return strings.Join(parts, "; ")
}

// Append 追加一个错误。
func (m *Multi) Append(err error) {
	if err != nil {
		m.Errors = append(m.Errors, err)
	}
}

// HasErrors 返回是否存在错误。
func (m *Multi) HasErrors() bool { return len(m.Errors) > 0 }

// Err 返回合并的 error。
func (m *Multi) Err() error {
	if len(m.Errors) == 0 {
		return nil
	}
	return m
}
