// Package errorsx 提供多错误聚合、链接与拆解。
package errorsx

import (
	"errors"
	"fmt"
	"strings"
)

// Multi 是多个错误的集合。
type Multi struct {
	errs []error
}

// New 创建一个空 Multi。
func New() *Multi { return &Multi{} }

// Append 添加一个错误。
func (m *Multi) Append(e error) {
	if e == nil {
		return
	}
	m.errs = append(m.errs, e)
}

// Errors 返回错误切片。
func (m *Multi) Errors() []error { return m.errs }

// HasError 判断是否有错误。
func (m *Multi) HasError() bool { return len(m.errs) > 0 }

// Error 实现 error 接口。
func (m *Multi) Error() string {
	if len(m.errs) == 0 {
		return "<no error>"
	}
	parts := make([]string, len(m.errs))
	for i, e := range m.errs {
		parts[i] = e.Error()
	}
	return fmt.Sprintf("%d errors: [%s]", len(m.errs), strings.Join(parts, "; "))
}

// Unwrap 返回首个错误（兼容 errors.Is/As）。
func (m *Multi) Unwrap() error {
	if len(m.errs) == 0 {
		return nil
	}
	return m.errs[0]
}

// First 返回第一个错误。
func (m *Multi) First() error {
	if len(m.errs) == 0 {
		return nil
	}
	return m.errs[0]
}

// Last 返回最后一个错误。
func (m *Multi) Last() error {
	if len(m.errs) == 0 {
		return nil
	}
	return m.errs[len(m.errs)-1]
}

// ToError 返回 error 接口（nil 表示无错）。
func (m *Multi) ToError() error {
	if len(m.errs) == 0 {
		return nil
	}
	return m
}

// Chain 是把多个 error 链接为链式错误（自带 next 指针）。
type Chain struct {
	cur error
}

// NewChain 创建错误链。
func NewChain(first error) *Chain { return &Chain{cur: first} }

// Append 向链尾追加一个错误。
func (c *Chain) Append(e error) *Chain {
	if e == nil {
		return c
	}
	if c.cur == nil {
		c.cur = e
		return c
	}
	c.cur = fmt.Errorf("%w | %w", c.cur, e)
	return c
}

// Err 返回链式错误。
func (c *Chain) Err() error { return c.cur }

// Is 简化 errors.Is 调用。
func Is(err, target error) bool { return errors.Is(err, target) }

// As 简化 errors.As 调用。
func As(err error, target any) bool { return errors.As(err, target) }
