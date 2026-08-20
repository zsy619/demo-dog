// Package recurring 提供一个简单的周期任务触发器。
// 与 scheduler 不同，本模块专注于"每隔固定间隔触发一次"的语义。
package recurring

import (
	"sync"
	"sync/atomic"
	"time"
)

// Job 是每次触发执行的任务。
type Job func(tick int64)

// Recurring 是一个周期触发器。
type Recurring struct {
	interval time.Duration
	job      Job

	mu     sync.Mutex
	stopCh chan struct{} // 本次运行的停止信号
	doneCh chan struct{} // 本次循环退出的信号
	tick   atomic.Int64
	closed atomic.Bool // 永久停止（防止 Stop 后再 Start）
}

// New 创建一个 Recurring。
func New(interval time.Duration, job Job) *Recurring {
	return &Recurring{
		interval: interval,
		job:      job,
	}
}

// Start 启动周期任务。已运行或已永久停止时返回 false。
func (r *Recurring) Start() bool {
	if r.closed.Load() {
		return false
	}
	r.mu.Lock()
	if r.stopCh != nil {
		r.mu.Unlock()
		return false
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	r.stopCh = stop
	r.doneCh = done
	r.mu.Unlock()
	go r.loop(stop, done)
	return true
}

// Stop 停止周期任务。停止或未运行返回 false。
func (r *Recurring) Stop() bool {
	r.mu.Lock()
	stop, done := r.stopCh, r.doneCh
	r.stopCh, r.doneCh = nil, nil
	r.mu.Unlock()
	if stop == nil {
		return false
	}
	close(stop)
	<-done
	return true
}

// Close 永久停止；之后 Start 永远返回 false。
func (r *Recurring) Close() {
	r.Stop()
	r.closed.Store(true)
}

// Tick 返回已触发的 tick 数。
func (r *Recurring) Tick() int64 {
	return r.tick.Load()
}

func (r *Recurring) loop(stop <-chan struct{}, done chan<- struct{}) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	defer close(done)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			n := r.tick.Add(1)
			r.job(n)
		}
	}
}
