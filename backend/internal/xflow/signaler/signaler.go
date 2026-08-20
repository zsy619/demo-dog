// Package signaler 实现计数信号量（channel-based）。
package signaler

import "context"

// Sem 是一个计数信号量。
type Sem struct {
	ch chan struct{}
}

// New 创建容量 n 的信号量。
func New(n int) *Sem {
	if n < 1 {
		n = 1
	}
	return &Sem{ch: make(chan struct{}, n)}
}

// Acquire 占用一个槽位（阻塞）。
func (s *Sem) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire 尝试非阻塞占用。
func (s *Sem) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release 释放一个槽位（空信号量上调用不阻塞，返回 false）。
func (s *Sem) Release() bool {
	select {
	case <-s.ch:
		return true
	default:
		return false
	}
}

// Available 返回当前可用槽位数。
func (s *Sem) Available() int {
	return cap(s.ch) - len(s.ch)
}

// Cap 返回容量。
func (s *Sem) Cap() int { return cap(s.ch) }

// Go 在信号量控制下运行 fn。
func (s *Sem) Go(ctx context.Context, fn func() error) error {
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()
	return fn()
}
