// Package throttle 提供节流（throttle）：限制一段时间内最多执行一次。
package throttle

import (
	"sync"
	"time"
)

// Throttle 在每次 Do 后强制等待 window。
type Throttle struct {
	mu       sync.Mutex
	window   time.Duration
	last     time.Time
}

// New 创建节流器。
func New(window time.Duration) *Throttle {
	if window <= 0 {
		window = time.Second
	}
	return &Throttle{window: window, last: time.Now().Add(-window)}
}

// Allow 返回 true 表示允许立刻执行；之后需要等待 window 才允许下一次。
func (t *Throttle) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if now.Sub(t.last) < t.window {
		return false
	}
	t.last = now
	return true
}

// Do 仅在 Allow 通过时执行 fn。
func (t *Throttle) Do(fn func()) bool {
	if !t.Allow() {
		return false
	}
	fn()
	return true
}

// Wait 阻塞直到允许下一次执行。
func (t *Throttle) Wait() {
	t.mu.Lock()
	elapsed := time.Since(t.last)
	t.mu.Unlock()
	if elapsed >= t.window {
		t.mu.Lock()
		t.last = time.Now()
		t.mu.Unlock()
		return
	}
	time.Sleep(t.window - elapsed)
	t.mu.Lock()
	t.last = time.Now()
	t.mu.Unlock()
}

// Window 返回节流窗口。
func (t *Throttle) Window() time.Duration { return t.window }
