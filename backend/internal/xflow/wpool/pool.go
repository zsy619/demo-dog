package wpool

// pool.go:Pool 主体与所有调度方法。
//
// Pool 按租户 FIFO 公平调度:每个租户独立的 channel 队列;worker
// 用 round-robin 从非空租户中取任务。

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Pool 是按租户 FIFO 公平调度的工作池。
type Pool struct {
	mu       sync.Mutex             // 保护 queues / order / inflight
	queues   map[string]chan Task   // tenant → 队列
	order    []string               // 租户首次出现顺序
	inflight map[string]int         // 当前在飞的租户计数
	workers  int                    // worker 数
	ctx      context.Context        // 全局 ctx,Close 时 cancel
	cancel   context.CancelFunc     // ctx 的 cancel
	wg       sync.WaitGroup         // 等待所有 worker 退出
	closed   atomic.Bool            // 是否已关闭
	run      atomic.Uint64          // 累计已执行任务数
	reject   atomic.Uint64          // 累计拒绝(队列满)次数
}

// ErrClosed 在 Close 之后调用 Submit 时返回。
var ErrClosed = errors.New("pool closed")

// ErrFull 在队列已满时返回。
var ErrFull = errors.New("queue full")

// New 创建带 workers 与每租户 queueSize 的 Pool。
//
// workers <= 0 时回退到 4;queueSize <= 0 时回退到 64。
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

// queueForLocked 返回指定租户的 channel;不存在则创建。
//
// 必须在持锁状态下调用。
func (p *Pool) queueForLocked(tenant string) chan Task {
	q, ok := p.queues[tenant]
	if !ok {
		q = make(chan Task, 64)
		p.queues[tenant] = q
	}
	return q
}

// Submit 将一个任务加入队列。
//
// 租户队列已满时返回 ErrFull(并递增 reject 计数)。
// Close 后提交返回 ErrClosed。
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

// worker round-robin 跨租户取任务并执行。
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

// next 从非空租户中按顺序取一个任务;没有任务时阻塞直到 ctx cancel 或新任务到达。
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
	// 同时发现尚未排序的租户。
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
	// 阻塞直到有消息到达。
	select {
	case <-p.ctx.Done():
		return Task{}, false
	case t := <-p.watch():
		return t, true
	}
}

// watch 合并所有租户队列到一个 channel,用于阻塞等待。
//
// 仅在持有 p.mu 时构造;merged 在 ctx cancel 时关闭。
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

// Close 排空队列并停止所有工作协程。
//
// 多次调用安全(幂等)。
func (p *Pool) Close() {
	if p.closed.CompareAndSwap(false, true) {
		p.cancel()
		p.wg.Wait()
	}
}
