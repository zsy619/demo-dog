// Package signalx 提供一个高粒度的信号量实现：
// - 计数信号量（可同时持有多份许可）
// - 超时等待
// - 不可用时立即返回错误
package signalx

import (
	"context"
	"errors"
)

// ErrExhausted 在请求数超过上限时返回。
var ErrExhausted = errors.New("signalx: 资源耗尽")

// Sem 是一个计数信号量。
type Sem struct {
	ch chan struct{}
}

// New 创建一个上限为 max 的信号量。
func New(max int) *Sem {
	if max <= 0 {
		max = 1
	}
	return &Sem{ch: make(chan struct{}, max)}
}

// Acquire 取一份许可。
func (s *Sem) Acquire() {
	s.ch <- struct{}{}
}

// Release 释放一份许可。
func (s *Sem) Release() {
	<-s.ch
}

// TryAcquire 尝试获取许可；不可用则返回 false。
func (s *Sem) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// AcquireCtx 在 ctx 取消或获得许可时返回。
func (s *Sem) AcquireCtx(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Capacity 返回总容量。
func (s *Sem) Capacity() int { return cap(s.ch) }

// InUse 返回当前占用数。
func (s *Sem) InUse() int { return len(s.ch) }

// Do 在许可内执行 fn。
func (s *Sem) Do(fn func()) {
	s.Acquire()
	defer s.Release()
	fn()
}

// DoCtx 在 ctx 内执行 fn；许可可用则执行，否则返回 ErrExhausted。
func (s *Sem) DoCtx(ctx context.Context, fn func()) error {
	if err := s.AcquireCtx(ctx); err != nil {
		return ErrExhausted
	}
	defer s.Release()
	fn()
	return nil
}
