// Package debounce 提供前端和后端通用的函数防抖与节流工具。
// Debounce 在最后一次触发后等待 wait 时长再执行；
// Throttle 在窗口期内最多执行一次。
package debounce

import (
	"sync"
	"sync/atomic"
	"time"
)

// Func 是被包装的函数签名。
type Func func()

// Debouncer 在每次 Trigger 时重置计时器，wait 之后才真正执行 fn。
type Debouncer struct {
	mu      sync.Mutex
	wait    time.Duration
	fn      Func
	timer   *time.Timer
	pending atomic.Bool
}

// New 创建一个 Debouncer。
func New(wait time.Duration, fn Func) *Debouncer {
	if wait <= 0 {
		wait = 100 * time.Millisecond
	}
	return &Debouncer{wait: wait, fn: fn}
}

// Trigger 触发一次调用，重置计时。
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.wait, func() {
		d.pending.Store(false)
		if d.fn != nil {
			d.fn()
		}
	})
	d.pending.Store(true)
	d.mu.Unlock()
}

// Pending 报告是否还有未执行任务。
func (d *Debouncer) Pending() bool { return d.pending.Load() }

// Cancel 取消尚未执行的调用。
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.pending.Store(false)
	d.mu.Unlock()
}

// Flush 立即执行未触发的任务。
func (d *Debouncer) Flush() {
	d.mu.Lock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.pending.Store(false)
	d.mu.Unlock()
	if d.fn != nil {
		d.fn()
	}
}

// Throttler 在窗口期内最多执行一次。
type Throttler struct {
	mu      sync.Mutex
	window  time.Duration
	fn      Func
	lastRun time.Time
}

// NewThrottle 创建一个 Throttler。
func NewThrottle(window time.Duration, fn Func) *Throttler {
	return &Throttler{window: window, fn: fn}
}

// Try 执行调用：若距上次执行已超过窗口，则执行并返回 true。
func (t *Throttler) Try() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if now.Sub(t.lastRun) < t.window {
		return false
	}
	t.lastRun = now
	if t.fn != nil {
		t.fn()
	}
	return true
}

// Reset 重置窗口。
func (t *Throttler) Reset() {
	t.mu.Lock()
	t.lastRun = time.Time{}
	t.mu.Unlock()
}
