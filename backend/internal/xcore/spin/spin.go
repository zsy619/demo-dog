// Package spin 提供快速自旋等待与自适应退避工具，
// 用于极短等待场景，如自旋锁、内存屏障前的等待。
package spin

import (
	"runtime"
	"sync/atomic"
	"time"
)

// Until 自旋直到 cond 返回 true 或 deadline 过期。
// 返回 cond 最终的值。
func Until(deadline time.Time, cond func() bool) bool {
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		runtime.Gosched()
	}
	return cond()
}

// Nanos 自旋执行 n 次 fn，期间不参与调度。
func Nanos(n int, fn func()) {
	for i := 0; i < n; i++ {
		fn()
	}
}

// Backoff 实现指数退避自旋等待。
type Backoff struct {
	attempt  int
	maxSpin  int
	maxTries int
	baseNs   int
	maxSleep time.Duration
}

// NewBackoff 构造一个退避器。
func NewBackoff() *Backoff {
	return &Backoff{maxSpin: 16, maxTries: 100, baseNs: 100, maxSleep: 100 * time.Microsecond}
}

// Do 自旋一次：根据 attempt 决定是否短暂 sleep。
// 达到 maxTries 仍不满足 cond 时返回 false（避免死循环）。
func (b *Backoff) Do(cond func() bool) bool {
	if cond() {
		return true
	}
	for b.attempt < b.maxTries {
		if cond() {
			return true
		}
		b.attempt++
		if b.attempt > b.maxSpin {
			shift := b.attempt - b.maxSpin
			if shift > 20 {
				shift = 20
			}
			wait := time.Duration(b.baseNs<<shift) * time.Nanosecond
			if b.maxSleep > 0 && wait > b.maxSleep {
				wait = b.maxSleep
			}
			time.Sleep(wait)
		} else {
			runtime.Gosched()
		}
	}
	return cond()
}

// Reset 重置退避器。
func (b *Backoff) Reset() { b.attempt = 0 }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WaitFor 在 timeout 内轮询检查 cond。
func WaitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	return Until(deadline, cond)
}

// PollingFlag 是一个原子轮询标志位。
type PollingFlag struct {
	v atomic.Bool
}

// Set 设置标志。
func (p *PollingFlag) Set(v bool) { p.v.Store(v) }

// Get 读取标志。
func (p *PollingFlag) Get() bool { return p.v.Load() }

// WaitUntilTrue 等待标志位变为 true 或超时。
func (p *PollingFlag) WaitUntilTrue(timeout time.Duration) bool {
	return WaitFor(timeout, p.Get)
}
