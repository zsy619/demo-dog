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
	stop     chan struct{}
	done     chan struct{}
	tick     atomic.Int64
	running  atomic.Bool
	mu       sync.Mutex
}

// New 创建一个 Recurring。
func New(interval time.Duration, job Job) *Recurring {
	return &Recurring{
		interval: interval,
		job:      job,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start 启动周期任务。
func (r *Recurring) Start() {
	r.mu.Lock()
	if r.running.Load() {
		r.mu.Unlock()
		return
	}
	r.running.Store(true)
	r.mu.Unlock()
	go r.loop()
}

// Stop 停止周期任务。
func (r *Recurring) Stop() {
	r.mu.Lock()
	if !r.running.Load() {
		r.mu.Unlock()
		return
	}
	r.running.Store(false)
	r.mu.Unlock()
	r.stop <- struct{}{}
	<-r.done
}

// Tick 返回已触发的 tick 数。
func (r *Recurring) Tick() int64 {
	return r.tick.Load()
}

func (r *Recurring) loop() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	defer close(r.done)
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			n := r.tick.Add(1)
			r.job(n)
		}
	}
}
