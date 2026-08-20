// Package poolg 提供一个固定大小的 goroutine 工作池。
// 任务通过有缓冲的 chan 提交，固定数量 worker 并发消费。
//
// 特性：
//   - 提交支持带 ctx 取消（SubmitCtx）和非阻塞尝试（TrySubmit）
//   - 任务 panic 不会中断 worker（panic 计数可读）
//   - 可注册 panic 处理器（OnPanic）
//   - Stats 暴露 submits/executed/panics/queue 实时计数
//   - Wait() 关闭队列并等待 worker 排空
//   - Close() 关闭池并等待 worker
package poolg

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Task 是被提交的工作单元。
type Task func()

// ErrClosed 在已关闭的池上提交时返回。
var ErrClosed = errors.New("poolg: 池已关闭")

// ErrQueueFull 在非阻塞提交且队列已满时返回。
var ErrQueueFull = errors.New("poolg: 队列已满")

// Pool 是一个固定大小的工作池。
//
// 零值不可用；请使用 New。
type Pool struct {
	workers int
	queue   chan Task
	wg      sync.WaitGroup

	closed  atomic.Bool
	closing atomic.Bool
	onPanic atomic.Pointer[func(name string, err error)]

	submits  atomic.Uint64
	executed atomic.Uint64
	panics   atomic.Uint64
	dropped  atomic.Uint64
}

// Stats 是池的运行时统计。
type Stats struct {
	Workers  int    `json:"workers"`
	Submits  uint64 `json:"submits"`
	Executed uint64 `json:"executed"`
	Panics   uint64 `json:"panics"`
	Dropped  uint64 `json:"dropped"`
	QueueLen int    `json:"queue_len"`
	QueueCap int    `json:"queue_cap"`
	Closed   bool   `json:"closed"`
}

// New 创建一个容量为 workers、队列深度为 queueSize 的池并启动 worker。
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
	noop := func(string, error) {}
	p.onPanic.Store(&noop)
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// OnPanic 注册任务 panic 处理器（默认静默）。
func (p *Pool) OnPanic(fn func(name string, err error)) {
	if fn == nil {
		fn = func(string, error) {}
	}
	p.onPanic.Store(&fn)
}

// Submit 提交一个任务；队列满时阻塞。池已关闭时返回 ErrClosed。
func (p *Pool) Submit(t Task) error {
	if t == nil {
		return nil
	}
	if p.closed.Load() {
		return ErrClosed
	}
	p.submits.Add(1)
	p.queue <- t
	return nil
}

// TrySubmit 非阻塞提交：队列满则丢弃并返回 ErrQueueFull。
func (p *Pool) TrySubmit(t Task) error {
	if t == nil {
		return nil
	}
	if p.closed.Load() {
		return ErrClosed
	}
	select {
	case p.queue <- t:
		p.submits.Add(1)
		return nil
	default:
		p.dropped.Add(1)
		return ErrQueueFull
	}
}

// SubmitCtx 在 ctx 取消时放弃提交。
func (p *Pool) SubmitCtx(ctx context.Context, t Task) error {
	if t == nil {
		return nil
	}
	if p.closed.Load() {
		return ErrClosed
	}
	select {
	case p.queue <- t:
		p.submits.Add(1)
		return nil
	case <-ctx.Done():
		p.dropped.Add(1)
		return ctx.Err()
	}
}

// Workers 返回 worker 数。
func (p *Pool) Workers() int { return p.workers }

// QueueLen 返回当前队列长度。
func (p *Pool) QueueLen() int { return len(p.queue) }

// QueueCap 返回队列容量。
func (p *Pool) QueueCap() int { return cap(p.queue) }

// Stats 返回当前统计快照。
func (p *Pool) Stats() Stats {
	return Stats{
		Workers:  p.workers,
		Submits:  p.submits.Load(),
		Executed: p.executed.Load(),
		Panics:   p.panics.Load(),
		Dropped:  p.dropped.Load(),
		QueueLen: len(p.queue),
		QueueCap: cap(p.queue),
		Closed:   p.closed.Load(),
	}
}

// Wait 关闭队列并等待所有 worker 排空（不接收新任务）。
// 多次调用安全。
func (p *Pool) Wait() {
	if !p.closing.CompareAndSwap(false, true) {
		p.wg.Wait()
		return
	}
	if p.closed.CompareAndSwap(false, true) {
		close(p.queue)
	}
	p.wg.Wait()
}

// Close 关闭池，等待 worker 排空。多次调用安全。
func (p *Pool) Close() {
	p.Wait()
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for t := range p.queue {
		p.runTask(t)
	}
}

func (p *Pool) runTask(t Task) {
	defer func() {
		if r := recover(); r != nil {
			p.panics.Add(1)
			if h := p.onPanic.Load(); h != nil {
				(*h)("poolg-task", fmt.Errorf("poolg task panic: %v", r))
			}
		}
	}()
	t()
	p.executed.Add(1)
}
