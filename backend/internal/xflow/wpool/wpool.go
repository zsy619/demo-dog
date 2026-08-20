// Package wpool worker 池：可配置缓冲队列的任务调度。
package wpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Task is a unit of work with a tenant tag for fairness.
type Task struct {
	Tenant string
	Run    func(ctx context.Context) error
}

// Pool is a worker pool with per-tenant FIFO fairness.
type Pool struct {
	mu       sync.Mutex
	queues   map[string]chan Task
	order    []string
	inflight map[string]int
	workers  int
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	closed   atomic.Bool
	run      atomic.Uint64
	reject   atomic.Uint64
}

// ErrClosed is returned when Submit is called after Close.
var ErrClosed = errors.New("pool closed")

// ErrFull is returned when the queue is full.
var ErrFull = errors.New("queue full")

// New creates a Pool with workers and per-tenant queueSize.
func New(workers, queueSize int) *Pool {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 64
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		queues:   make(map[string]chan Task),
		inflight: make(map[string]int),
		workers:  workers,
		ctx:      ctx,
		cancel:   cancel,
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	p.queueForLocked("*default")
	p.order = append(p.order, "*default")
	return p
}

func (p *Pool) queueForLocked(tenant string) chan Task {
	q, ok := p.queues[tenant]
	if !ok {
		q = make(chan Task, 64)
		p.queues[tenant] = q
	}
	return q
}

// Submit enqueues a task. Returns ErrFull if the tenant
// queue is full.
func (p *Pool) Submit(t Task) error {
	if p.closed.Load() {
		return ErrClosed
	}
	if t.Tenant == "" {
		t.Tenant = "*default"
	}
	p.mu.Lock()
	q, ok := p.queues[t.Tenant]
	if !ok {
		q = make(chan Task, 64)
		p.queues[t.Tenant] = q
		p.order = append(p.order, t.Tenant)
	}
	p.mu.Unlock()
	select {
	case q <- t:
		return nil
	default:
		p.reject.Add(1)
		return ErrFull
	}
}

// worker pulls tasks round-robin across tenants.
func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		t, ok := p.next()
		if !ok {
			return
		}
		p.run.Add(1)
		_ = t.Run(p.ctx)
	}
}

func (p *Pool) next() (Task, bool) {
	p.mu.Lock()
	tenants := make([]string, 0, len(p.queues))
	visited := make(map[string]bool)
	for _, t := range p.order {
		if visited[t] {
			continue
		}
		visited[t] = true
		if q, ok := p.queues[t]; ok && len(q) > 0 {
			tenants = append(tenants, t)
		}
	}
	// Also discover any tenants not yet in order.
	for t := range p.queues {
		if !visited[t] {
			visited[t] = true
			if q := p.queues[t]; q != nil && len(q) > 0 {
				tenants = append(tenants, t)
			}
		}
	}
	p.mu.Unlock()
	for _, t := range tenants {
		p.mu.Lock()
		q := p.queues[t]
		p.mu.Unlock()
		select {
		case task := <-q:
			return task, true
		default:
		}
	}
	if p.closed.Load() {
		return Task{}, false
	}
	// Block until something arrives.
	select {
	case <-p.ctx.Done():
		return Task{}, false
	case t := <-p.watch():
		return t, true
	}
}

func (p *Pool) watch() <-chan Task {
	p.mu.Lock()
	defer p.mu.Unlock()
	merged := make(chan Task)
	for _, q := range p.queues {
		go func(q chan Task) {
			for t := range q {
				merged <- t
			}
		}(q)
	}
	go func() {
		<-p.ctx.Done()
		close(merged)
	}()
	return merged
}

// Close drains the queue and stops workers.
func (p *Pool) Close() {
	if p.closed.CompareAndSwap(false, true) {
		p.cancel()
		p.wg.Wait()
	}
}

// Stats returns counters.
type Stats struct {
	Workers  int    `json:"workers"`
	Tenants  int    `json:"tenants"`
	Run      uint64 `json:"run"`
	Rejected uint64 `json:"rejected"`
}

// Stats returns the snapshot.
func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Stats{
		Workers: p.workers, Tenants: len(p.queues),
		Run: p.run.Load(), Rejected: p.reject.Load(),
	}
}
