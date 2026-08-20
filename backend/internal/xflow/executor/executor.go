// Package executor 提供固定 worker 数量的任务执行器：
// 有缓冲任务队列，支持非阻塞与阻塞提交，\n// 任务 panic 不会中断 worker。
//
// 用法：
//
//	ex := executor.New(4, 1024)
//	defer ex.Close()
//	ex.Submit(func() { ... })
package executor

import (
	"context"
	"sync"
	"sync/atomic"
)

// Job 是一次任务。
type Job func()

// Executor 是固定 worker 的执行器。
type Executor struct {
	jobs    chan Job
	closed  atomic.Bool
	closeMu sync.Mutex // 保护 close 操作
	wg      sync.WaitGroup
	workers int

	queueLen atomic.Int64 // 待执行任务数（语义：ch 中的任务）
	executed atomic.Uint64
	panics   atomic.Uint64
}

// Stats 是执行器统计。
type Stats struct {
	Workers  int    `json:"workers"`
	QueueLen int64  `json:"queue_len"`
	QueueCap int    `json:"queue_cap"`
	Executed uint64 `json:"executed"`
	Panics   uint64 `json:"panics"`
	Closed   bool   `json:"closed"`
}

// New 创建一个带 n worker、队列长度 qLen 的 Executor。
// workers < 1 视为 1，qLen < 1 视为 1024。
func New(workers, qLen int) *Executor {
	if workers < 1 {
		workers = 1
	}
	if qLen < 1 {
		qLen = 1024
	}
	e := &Executor{workers: workers, jobs: make(chan Job, qLen)}
	for i := 0; i < workers; i++ {
		e.wg.Add(1)
		go e.run()
	}
	return e
}

// Submit 非阻塞提交任务；返回是否成功。
func (e *Executor) Submit(j Job) bool {
	if j == nil || e.closed.Load() {
		return false
	}
	select {
	case e.jobs <- j:
		e.queueLen.Add(1)
		return true
	default:
		return false
	}
}

// SubmitBlocking 阻塞提交任务；队列满时阻塞直到可入队。
// 已关闭时返回 false。
func (e *Executor) SubmitBlocking(j Job) bool {
	if j == nil || e.closed.Load() {
		return false
	}
	e.jobs <- j
	e.queueLen.Add(1)
	return true
}

// SubmitCtx 在 ctx 取消时放弃提交。
func (e *Executor) SubmitCtx(ctx context.Context, j Job) bool {
	if j == nil || e.closed.Load() {
		return false
	}
	select {
	case e.jobs <- j:
		e.queueLen.Add(1)
		return true
	case <-ctx.Done():
		return false
	}
}

// Close 关闭执行器，等待所有 worker 完成。幂等。
// 在调用 Close 之后不能再 Submit。
func (e *Executor) Close() {
	e.closeMu.Lock()
	defer e.closeMu.Unlock()
	if e.closed.Swap(true) {
		return
	}
	close(e.jobs)
	e.wg.Wait()
}

// Workers 返回 worker 数。
func (e *Executor) Workers() int { return e.workers }

// QueueLen 返回当前队列长度。
func (e *Executor) QueueLen() int64 { return e.queueLen.Load() }

// QueueCap 返回队列容量。
func (e *Executor) QueueCap() int { return cap(e.jobs) }

// Stats 返回统计快照。
func (e *Executor) Stats() Stats {
	return Stats{
		Workers:  e.workers,
		QueueLen: e.queueLen.Load(),
		QueueCap: cap(e.jobs),
		Executed: e.executed.Load(),
		Panics:   e.panics.Load(),
		Closed:   e.closed.Load(),
	}
}

func (e *Executor) run() {
	defer e.wg.Done()
	for j := range e.jobs {
		e.runJob(j)
	}
}

func (e *Executor) runJob(j Job) {
	defer func() {
		if r := recover(); r != nil {
			e.panics.Add(1)
		}
		e.executed.Add(1)
		e.queueLen.Add(-1)
	}()
	j()
}
