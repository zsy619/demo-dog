// Package batch implements a bounded worker pool for the OTLP ingest pipeline.
//
// The Collector-style feature set the demo targets needs three things from the
// write side:
//
//  1. Throughput: many small writes must be coalesced into fewer larger ones.
//  2. Backpressure: when the engine is hot, ingest must shed load, not block.
//  3. Retries: transient failures (e.g. a bucket full) must be retried with
//     exponential backoff before giving up.
//
// Pool provides a small generic worker pool that satisfies all three.
package batch

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Job 是提交到池的工作单元。Done 恰好被调用一次
// to signal completion (success or failure). Error is reported back through
// the optional Results callback.
type Job struct {
	Payload any
	Fn      func(ctx context.Context, payload any) error
	Done    func(err error)
}

// Pool 是带协作背压的有界工作池。
type Pool struct {
	workers int
	queue   chan Job
	wg      sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	retryMax     int
	retryBackoff time.Duration

	// 通过 Stats() 暴露的簿记。
	accepted atomic.Int64
	processed atomic.Int64
	retried   atomic.Int64
	failed    atomic.Int64

	rng *rand.Rand
	mu  sync.Mutex
}

// Options 用于配置新 Pool。
type Options struct {
	Workers     int
	QueueSize   int
	RetryMax    int
	RetryBackoff time.Duration
}

// NewPool 以给定选项创建 Pool 并启动其 workers。
func NewPool(opts Options) *Pool {
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 1024
	}
	if opts.RetryMax <= 0 {
		opts.RetryMax = 3
	}
	if opts.RetryBackoff <= 0 {
		opts.RetryBackoff = 50 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers:      opts.Workers,
		queue:        make(chan Job, opts.QueueSize),
		ctx:          ctx,
		cancel:       cancel,
		retryMax:     opts.RetryMax,
		retryBackoff: opts.RetryBackoff,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for i := 0; i < opts.Workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// Submit enqueues a job. Returns false if the queue is full (backpressure).
func (p *Pool) Submit(j Job) bool {
	select {
	case p.queue <- j:
		p.accepted.Add(1)
		return true
	default:
		return false
	}
}

// Close drains the queue and waits for workers to exit.
func (p *Pool) Close() {
	close(p.queue)
	p.cancel()
	p.wg.Wait()
}

// Stats 返回池计数器的快照。
type Stats struct {
	Accepted  int64 `json:"accepted"`
	Processed int64 `json:"processed"`
	Retried   int64 `json:"retried"`
	Failed    int64 `json:"failed"`
	QueueLen  int   `json:"queue_len"`
	Workers   int   `json:"workers"`
}

// Stats 返回实时计数器和当前队列深度。
func (p *Pool) Stats() Stats {
	return Stats{
		Accepted:  p.accepted.Load(),
		Processed: p.processed.Load(),
		Retried:   p.retried.Load(),
		Failed:    p.failed.Load(),
		QueueLen:  len(p.queue),
		Workers:   p.workers,
	}
}

// worker consumes jobs from the queue and runs them with retry semantics.
func (p *Pool) worker() {
	defer p.wg.Done()
	for j := range p.queue {
		p.processWithRetry(j)
	}
}

// processWithRetry runs the job, applying exponential backoff with jitter on errors.
func (p *Pool) processWithRetry(j Job) {
	var err error
	for attempt := 0; attempt <= p.retryMax; attempt++ {
		if attempt > 0 {
			p.retried.Add(1)
			backoff := p.retryBackoff * (1 << (attempt - 1))
			jitter := time.Duration(p.jitterInt(50)) * time.Millisecond
			select {
			case <-p.ctx.Done():
				err = p.ctx.Err()
				if j.Done != nil {
					j.Done(err)
				}
				p.failed.Add(1)
				return
			case <-time.After(backoff + jitter):
			}
		}
		err = j.Fn(p.ctx, j.Payload)
		if err == nil {
			p.processed.Add(1)
			if j.Done != nil {
				j.Done(nil)
			}
			return
		}
	}
	p.failed.Add(1)
	if j.Done != nil {
		j.Done(err)
	}
}

// jitterInt returns a uniform random integer in [0, max).
func (p *Pool) jitterInt(max int) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rng.Intn(max)
}
