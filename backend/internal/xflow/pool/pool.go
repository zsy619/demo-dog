// Package pool 通用 goroutine 池：固定大小工作协程池。
package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Job 是工作单元。
type Job struct {
	Name string
	Run  func(ctx context.Context) error
}

// Result 是一次 Job 的结果。
type Result struct {
	Job    string
	Err    error
	Panic  any
}

// Pool 是固定大小、有界队列的工作池，并
// 背压. When the 队列 is full Submit 返回 an
// error rather than blocking, so callers can shed load.
type Pool struct {
	name      string
	workers   int
	queueCap  int
	runOn     atomic.Bool
	jobs      chan Job
	results   chan Result
	wg        sync.WaitGroup
	dropped   atomic.Uint64
	accepted  atomic.Uint64
	completed atomic.Uint64
	failed    atomic.Uint64
	panicked  atomic.Uint64
	onPanic   func(name string, r any)
	ctx       context.Context
	cancel    context.CancelFunc
}

// Config 用于配置池。
type Config struct {
	Name      string
	Workers   int
	QueueCap  int
	OnPanic   func(name string, r any)
}

// New 创建一个新池。在之前池不会启动
// Start() is called.
func New(cfg Config) *Pool {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.QueueCap <= 0 {
		cfg.QueueCap = cfg.Workers * 16
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		name:     cfg.Name,
		workers:  cfg.Workers,
		queueCap: cfg.QueueCap,
		onPanic:  cfg.OnPanic,
		jobs:     make(chan Job, cfg.QueueCap),
		results:  make(chan Result, cfg.QueueCap),
		ctx:      ctx,
		cancel:   cancel,
	}
	return p
}

// Start 启动 worker goroutine。
func (p *Pool) Start() {
	if !p.runOn.CompareAndSwap(false, true) {
		return
	}
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// Stop 通知 worker 退出并等待它们。
// Cancels the context so in-flight jobs see Done.
func (p *Pool) Stop() {
	if !p.runOn.Load() {
		return
	}
	p.cancel()
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
}

// Submit 将任务入队。当队列处于
// capacity so callers can shed load rather than block.
func (p *Pool) Submit(j Job) error {
	if !p.runOn.Load() {
		return errors.New("pool not running")
	}
	select {
	case p.jobs <- j:
		p.accepted.Add(1)
		return nil
	default:
		p.dropped.Add(1)
		return ErrFull
	}
}

// SubmitCtx 是阻塞变体。ctx.Done() 返回 ErrFull
// early so callers can cancel their wait.
func (p *Pool) SubmitCtx(ctx context.Context, j Job) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.jobs <- j:
		p.accepted.Add(1)
		return nil
	default:
		p.dropped.Add(1)
		return ErrFull
	}
}

// Results 暴露结果 channel。
func (p *Pool) Results() <-chan Result { return p.results }

// Stats 返回池计数器的快照。
type Stats struct {
	Name      string `json:"name"`
	Workers   int    `json:"workers"`
	QueueCap  int    `json:"queue_cap"`
	QueueLen  int    `json:"queue_len"`
	Accepted  uint64 `json:"accepted"`
	Dropped   uint64 `json:"dropped"`
	Completed uint64 `json:"completed"`
	Failed    uint64 `json:"failed"`
	Panicked  uint64 `json:"panicked"`
}

// Stats 返回计数器。
func (p *Pool) Stats() Stats {
	return Stats{
		Name:      p.name,
		Workers:   p.workers,
		QueueCap:  p.queueCap,
		QueueLen:  len(p.jobs),
		Accepted:  p.accepted.Load(),
		Dropped:   p.dropped.Load(),
		Completed: p.completed.Load(),
		Failed:    p.failed.Load(),
		Panicked:  p.panicked.Load(),
	}
}

// ErrFull 在队列已满时由 Submit 返回。
var ErrFull = errors.New("pool queue full")

func (p *Pool) worker() {
	defer p.wg.Done()
	for j := range p.jobs {
		p.run(j)
	}
}

func (p *Pool) run(j Job) {
	resCh := p.results
	defer func() {
		if r := recover(); r != nil {
			p.panicked.Add(1)
			if p.onPanic != nil {
				p.onPanic(j.Name, r)
			}
			select {
			case resCh <- Result{Job: j.Name, Panic: r}:
			default:
			}
		}
	}()
	err := j.Run(p.ctx)
	if err != nil {
		p.failed.Add(1)
	} else {
		p.completed.Add(1)
	}
	select {
	case resCh <- Result{Job: j.Name, Err: err}:
	default:
	}
}
