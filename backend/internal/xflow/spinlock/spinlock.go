// Package spinlock 提供基于 atomic 的简单自旋锁。
// 适用于持有时间非常短的临界区。
package spinlock

import (
	"runtime"
	"sync/atomic"
)

// SpinLock 是 32-bit 自旋锁。
type SpinLock struct {
	state atomic.Uint32
}

// Lock 自旋获取锁。
func (s *SpinLock) Lock() {
	for !s.TryLock() {
		runtime.Gosched()
	}
}

// TryLock 尝试获取锁。
func (s *SpinLock) TryLock() bool {
	return s.state.CompareAndSwap(0, 1)
}

// Unlock 释放锁。
func (s *SpinLock) Unlock() {
	s.state.Store(0)
}

// Do 执行 fn，持有锁。
func (s *SpinLock) Do(fn func()) {
	s.Lock()
	defer s.Unlock()
	fn()
}
