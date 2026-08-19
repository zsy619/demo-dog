// Package barrier 提供一次性屏障同步原语：
// - CountBarrier：等待 N 个 Done 或 Cancel
// - TimeBarrier：阻塞到指定时间点或 Cancel
package barrier

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// CountBarrier 是 N 个 Done 的屏障。
type CountBarrier struct {
	target  int
	mu      sync.Mutex
	done    int
	closed  chan struct{}
	cancelC chan struct{}
	reached atomic.Bool
}

// NewCount 创建一个等待 target 个 Done 的屏障。
func NewCount(target int) *CountBarrier {
	if target <= 0 {
		target = 1
	}
	return &CountBarrier{
		target:  target,
		closed:  make(chan struct{}),
		cancelC: make(chan struct{}),
	}
}

// Done 增加一次计数，达 target 后关闭 closeCh。
func (b *CountBarrier) Done() {
	b.mu.Lock()
	if b.reached.Load() {
		b.mu.Unlock()
		return
	}
	b.done++
	done := b.done >= b.target
	if done {
		b.reached.Store(true)
		close(b.closed)
	}
	b.mu.Unlock()
}

// Wait 阻塞直到屏障到达或取消。
func (b *CountBarrier) Wait(ctx context.Context) bool {
	select {
	case <-b.closed:
		return true
	case <-b.cancelC:
		return false
	case <-ctx.Done():
		return false
	}
}

// Cancel 主动取消屏障。
func (b *CountBarrier) Cancel() {
	b.mu.Lock()
	if !b.reached.Load() {
		b.reached.Store(true)
		close(b.cancelC)
	}
	b.mu.Unlock()
}

// DoneCount 返回当前 Done 调用次数。
func (b *CountBarrier) DoneCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.done
}

// TimeBarrier 阻塞到指定时刻或被取消。
type TimeBarrier struct {
	time    time.Time
	cancel  chan struct{}
	reached atomic.Bool
}

// NewTime 创建一个阻塞到 t 的屏障。
func NewTime(t time.Time) *TimeBarrier {
	return &TimeBarrier{time: t, cancel: make(chan struct{})}
}

// Wait 阻塞到指定时刻或被取消。
func (b *TimeBarrier) Wait() bool {
	t := time.NewTimer(time.Until(b.time))
	defer t.Stop()
	select {
	case <-t.C:
		b.reached.Store(true)
		return true
	case <-b.cancel:
		return false
	}
}

// Cancel 主动取消时间屏障。
func (b *TimeBarrier) Cancel() {
	b.mu()
	close(b.cancel)
}

// fake lock helper for cancel
func (b *TimeBarrier) mu() {}
