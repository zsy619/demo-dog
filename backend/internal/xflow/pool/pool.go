package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Job is the unit of work.
type Job struct {
	Name string
	Run  func(ctx context.Context) error
}

// Result is the outcome of one Job.
type Result struct {
	Job    string
	Err    error
	Panic  any
}

// Pool is a fixed-size worker pool with bounded queue and
// backpressure. When the queue is full Submit returns an
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

// Config configures the pool.
type Config struct {
	Name      string
	Workers   int
	QueueCap  int
	OnPanic   func(name string, r any)
}

// New creates a new pool. The pool does not start until
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

// Start launches the worker goroutines.
func (p *Pool) Start() {
	if !p.runOn.CompareAndSwap(false, true) {
		return
	}
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// Stop signals workers to exit and waits for them.
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

// Submit enqueues a job. Returns ErrFull when the queue is at
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

// SubmitCtx is the blocking variant. ctx.Done() returns ErrFull
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

// Results exposes the result channel.
func (p *Pool) Results() <-chan Result { return p.results }

// Stats returns a snapshot of pool counters.
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

// Stats returns the counters.
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

// ErrFull is returned by Submit when the queue is full.
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
