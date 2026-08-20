// Package semaphore 提供加权计数信号量：
// 适合资源访问限流（如并发连接数、内存配额等）。
//
// 特性：
//   - 加权获取（Acquire(n)）
//   - 支持 ctx 取消
//   - 不同 weight 的等待者可被正确唤醒（按 FIFO 顺序）
//   - 多种调用形式：Acquire/TryAcquire/Do
package semaphore

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrExhausted 在 ctx 取消时返回。
var ErrExhausted = errors.New("semaphore: 已耗尽")

// ErrInvalidWeight 在 weight <= 0 时返回。
var ErrInvalidWeight = errors.New("semaphore: weight 必须 > 0")

// ErrTooLarge 在 weight 超过 max 时返回。
var ErrTooLarge = errors.New("semaphore: weight 超过 max")

// Weighted 是加权信号量。
//
// 内部维护"已分配权重和"（cur），最多为 max。
// 当 Acquire 时 cur + n > max，将 waiter 加入队列并阻塞。
// Release 时 cur -= n；然后按 FIFO 唤醒能容纳的等待者。
type Weighted struct {
	mu      sync.Mutex
	max     int64
	cur     int64
	waiters []waiter
}

type waiter struct {
	weight int64
	ch     chan struct{}
}

// NewWeighted 创建一个最大权重为 max 的信号量。
// max <= 0 视为 1。
func NewWeighted(max int64) *Weighted {
	if max <= 0 {
		max = 1
	}
	return &Weighted{max: max}
}

// Acquire 阻塞直到获得权重 n；ctx 取消返回 ErrExhausted。
// n > max 立即返回 ErrTooLarge；n <= 0 立即返回 nil。
func (s *Weighted) Acquire(ctx context.Context, n int64) error {
	if n <= 0 {
		return nil
	}
	if n > s.max {
		return ErrTooLarge
	}
	s.mu.Lock()
	if s.cur+n <= s.max {
		s.cur += n
		s.mu.Unlock()
		return nil
	}
	ch := make(chan struct{}, 1)
	w := waiter{weight: n, ch: ch}
	s.waiters = append(s.waiters, w)
	s.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		s.removeWaiter(w)
		s.mu.Unlock()
		return ErrExhausted
	}
}

func (s *Weighted) removeWaiter(w waiter) {
	for i, x := range s.waiters {
		if x.ch == w.ch {
			s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
			return
		}
	}
}

// Release 释放 n 个权重；按 FIFO 唤醒尽可能多的等待者。
// 即使没有等待者，cur 也会相应减少。
func (s *Weighted) Release(n int64) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 先扣减 cur
	s.cur -= n
	if s.cur < 0 {
		s.cur = 0
	}
	// 然后尝试唤醒等待者：每唤醒一个 waiter，相当于该 waiter 立刻占用 weight
	for len(s.waiters) > 0 {
		w := s.waiters[0]
		if s.cur+w.weight > s.max {
			break // 当前余量不够唤醒队首
		}
		s.waiters = s.waiters[1:]
		s.cur += w.weight // 占用
		w.ch <- struct{}{}
	}
}

// TryAcquire 非阻塞尝试获取。
func (s *Weighted) TryAcquire(n int64) bool {
	if n <= 0 {
		return true
	}
	if n > s.max {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur+n <= s.max {
		s.cur += n
		return true
	}
	return false
}

// Available 返回剩余权重。
func (s *Weighted) Available() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.max - s.cur
}

// Waiters 返回当前等待者数量。
func (s *Weighted) Waiters() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.waiters)
}

// Do 在自动 Acquire/Release 中执行 fn；Acquire 失败不 Release。
func (s *Weighted) Do(ctx context.Context, n int64, fn func() error) error {
	if err := s.Acquire(ctx, n); err != nil {
		return err
	}
	err := fn()
	s.Release(n)
	return err
}

// Sleep 是 Acquire 的阻塞便捷包装（ctx 100ms 超时，仅作演示）。
// 不应在生产代码使用——明确传 ctx 更安全。
func (s *Weighted) Sleep(n int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.Acquire(ctx, n); err != nil {
		return // 已耗尽时不释放
	}
	s.Release(n)
}
