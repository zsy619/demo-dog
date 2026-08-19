// Package poolg 提供一个固定大小的 goroutine 池，
// 与 errgroup 类似但显式提供 worker 与 queue 管理。
package poolg

import (
	"context"
	"sync"
	"sync/atomic"
)

// Task 表示一个待执行的工作单元。
type Task func()

// Pool 是一个固定大小的工作池。
type Pool struct {
	mu       sync.Mutex
	workers  int
	queue    chan Task
	wg       sync.WaitGroup
	closed   atomic.Bool
	submits  atomic.Uint64
	executed atomic.Uint64
}

// New 创建一个容量为 workers 的池。
func New(workers, queueSize int) *Pool {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = workers * 4
	}
	p := &Pool{
		workers: workers,
		queue:   make(chan Task, queueSize),
	}
	p.start()
	return p
}

func (p *Pool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for t := range p.queue {
		func() {
			defer func() { _ = recover() }()
			t()
		}()
		p.executed.Add(1)
	}
}

// Submit 提交一个任务。
func (p *Pool) Submit(t Task) {
	if p.closed.Load() {
		return
	}
	p.submits.Add(1)
	p.queue <- t
}

// SubmitCtx 在 ctx 取消时放弃提交。
func (p *Pool) SubmitCtx(ctx context.Context, t Task) bool {
	if p.closed.Load() {
		return false
	}
	select {
	case p.queue <- t:
		p.submits.Add(1)
		return true
	case <-ctx.Done():
		return false
	}
}

// Wait 等待所有任务完成。
func (p *Pool) Wait() {
	p.mu.Lock()
	if !p.closed.Load() {
		p.closed.Store(true)
		close(p.queue)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// Stats 返回计数器快照。
type Stats struct {
	Submits  uint64 `json:"submits"`
	Executed uint64 `json:"executed"`
	QueueLen int    `json:"queue_len"`
}

// Stats 返回当前计数。
func (p *Pool) Stats() Stats {
	return Stats{
		Submits:  p.submits.Load(),
		Executed: p.executed.Load(),
		QueueLen: len(p.queue),
	}
}
