// Package executor 提供固定 worker 数量的任务执行器。
package executor

import (
	"sync"
	"sync/atomic"
)

// Job 是一次任务。
type Job func()

// Executor 是固定 worker 的执行器。
type Executor struct {
	mu       sync.Mutex
	jobs     chan Job
	closed   atomic.Bool
	wg       sync.WaitGroup
	workers  int
	queueLen atomic.Int64
}

// New 创建一个带 n worker、队列长度 qLen 的 Executor。
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

// Submit 提交一个任务。
func (e *Executor) Submit(j Job) bool {
	if e.closed.Load() {
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

// SubmitBlocking 阻塞提交任务。
func (e *Executor) SubmitBlocking(j Job) {
	if e.closed.Load() {
		return
	}
	e.jobs <- j
	e.queueLen.Add(1)
}

// Close 关闭执行器，等待所有 worker 完成。
func (e *Executor) Close() {
	if e.closed.Swap(true) {
		return
	}
	close(e.jobs)
	e.wg.Wait()
}

// QueueLen 返回当前队列长度。
func (e *Executor) QueueLen() int64 { return e.queueLen.Load() }

func (e *Executor) run() {
	defer e.wg.Done()
	for j := range e.jobs {
		j()
		e.queueLen.Add(-1)
	}
}
