// Package semaphore 提供一个有错误返回的加权计数信号量：
// 适合资源访问限流。
package semaphore

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrExhausted 在 ctx 取消时返回。
var ErrExhausted = errors.New("semaphore: 已耗尽")

// Weighted 是加权信号量。
type Weighted struct {
	mu      sync.Mutex
	max     int64
	cur     int64
	waiters []chan struct{}
}

// NewWeighted 创建一个最大权重为 max 的信号量。
func NewWeighted(max int64) *Weighted {
	if max <= 0 {
		max = 1
	}
	return &Weighted{max: max}
}

// Acquire 阻塞直到获得权重 n；ctx 取消返回 ErrExhausted。
func (s *Weighted) Acquire(ctx context.Context, n int64) error {
	if n <= 0 {
		return nil
	}
	s.mu.Lock()
	if s.cur+n <= s.max {
		s.cur += n
		s.mu.Unlock()
		return nil
	}
	ch := make(chan struct{}, 1)
	s.waiters = append(s.waiters, ch)
	s.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		// 移除等待
		for i, c := range s.waiters {
			if c == ch {
				s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		return ErrExhausted
	}
}

// Release 释放 n 个权重。
func (s *Weighted) Release(n int64) {
	s.mu.Lock()
	s.cur -= n
	if s.cur < 0 {
		s.cur = 0
	}
	// 唤醒一个等待者
	for len(s.waiters) > 0 {
		ch := s.waiters[0]
		s.waiters = s.waiters[1:]
		s.cur += n
		if s.cur > s.max {
			s.cur -= n
			// 放回去
			s.waiters = append([]chan struct{}{ch}, s.waiters...)
			break
		}
		ch <- struct{}{}
		return
	}
	s.mu.Unlock()
}

// TryAcquire 非阻塞尝试获取。
func (s *Weighted) TryAcquire(n int64) bool {
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

// Do 在自动 Acquire/Release 中执行 fn。
func (s *Weighted) Do(ctx context.Context, n int64, fn func() error) error {
	if err := s.Acquire(ctx, n); err != nil {
		return err
	}
	err := fn()
	s.Release(n)
	return err
}

// Sleep 是 Acquire 的阻塞便捷包装。
func (s *Weighted) Sleep(n int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = s.Acquire(ctx, n)
	s.Release(n)
}
