// Package bulkhead 隔板限流：通过并发上限隔离不同 key 集合的负载。
package bulkhead

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Bulkhead 是基于信号量的并发限流器。
// 在执行临界区之前获取许可证，执行完毕之后释放。
// Bulkhead 维护计数器，记录许可证被占满的次数，以便调用方进行负载削减。
type Bulkhead struct {
	name    string
	max     int
	cur     atomic.Int64
	pending atomic.Int64
	mu      sync.Mutex
	sem     chan struct{}
	acquired atomic.Uint64
	rejected atomic.Uint64
	released atomic.Uint64
	timeouts atomic.Uint64
}

// ErrFull 在 Acquire 没有可立即使用的许可证时返回。
var ErrFull = errors.New("bulkhead full")

// New 构造一个允许最大并发许可证数为 max 的 Bulkhead。
func New(name string, max int) *Bulkhead {
	if max <= 0 {
		max = 1
	}
	return &Bulkhead{
		name: name,
		max:  max,
		sem:  make(chan struct{}, max),
	}
}

// Acquire 获取一个许可证。若无空闲许可证则立即返回 ErrFull（调用方应进行负载削减）。
func (b *Bulkhead) Acquire() error {
	if int64(len(b.sem)) >= int64(b.max) {
		b.rejected.Add(1)
		return ErrFull
	}
	b.sem <- struct{}{}
	b.cur.Add(1)
	b.pending.Add(1)
	b.acquired.Add(1)
	return nil
}

// AcquireCtx 在许可证空闲或 ctx 完成之前阻塞。
// 超时时返回 ctx.Err() 并增加超时计数。
func (b *Bulkhead) AcquireCtx(ctx context.Context) error {
	select {
	case b.sem <- struct{}{}:
		b.cur.Add(1)
		b.pending.Add(1)
		b.acquired.Add(1)
		return nil
	case <-ctx.Done():
		b.rejected.Add(1)
		return ctx.Err()
	}
}

// Release 释放一个许可证。每次 Acquire 成功后必须且只能调用一次。
// 若当前未持有许可证，则为空操作（返回 false）。
func (b *Bulkhead) Release() bool {
	select {
	case <-b.sem:
		b.cur.Add(-1)
		b.pending.Add(-1)
		b.released.Add(1)
		return true
	default:
		return false
	}
}

// Run 在持有许可证的情况下执行 op。
// 若无许可证可用则返回 ErrFull。
func (b *Bulkhead) Run(op func() error) error {
	if err := b.Acquire(); err != nil {
		return err
	}
	defer b.Release()
	return op()
}

// RunCtx 使用 ctx 获取许可证后执行 op。
func (b *Bulkhead) RunCtx(ctx context.Context, op func() error) error {
	if err := b.AcquireCtx(ctx); err != nil {
		return err
	}
	defer b.Release()
	return op()
}

// Stats 表示 Bulkhead 的状态。
type Stats struct {
	Name     string `json:"name"`
	Max      int    `json:"max"`
	Current  int64  `json:"current"`
	Pending  int64  `json:"pending"`
	Acquired uint64 `json:"acquired"`
	Released uint64 `json:"released"`
	Rejected uint64 `json:"rejected"`
	Timeouts uint64 `json:"timeouts"`
}

// Stats 返回当前状态的快照。
func (b *Bulkhead) Stats() Stats {
	return Stats{
		Name:     b.name,
		Max:      b.max,
		Current:  b.cur.Load(),
		Pending:  b.pending.Load(),
		Acquired: b.acquired.Load(),
		Released: b.released.Load(),
		Rejected: b.rejected.Load(),
		Timeouts: b.timeouts.Load(),
	}
}

// Max 返回配置的许可证数量。
func (b *Bulkhead) Max() int { return b.max }

// Name 返回 Bulkhead 的名称。
func (b *Bulkhead) Name() string { return b.name }
