// Package bulkhead 隔板限流：通过并发上限隔离不同 key 集合的负载。
package bulkhead

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Bulkhead is a semaphore-based concurrency limiter.
// Permits are acquired before a critical section runs and
// released after. The bulkhead keeps counters for the number
// of times it was full so callers can shed load.
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

// ErrFull is returned by Acquire when no permit is
// immediately available.
var ErrFull = errors.New("bulkhead full")

// New constructs a bulkhead with max concurrent permits.
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

// Acquire takes one permit. If no permit is free, returns
// ErrFull immediately (the caller should shed load).
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

// AcquireCtx blocks until a permit is free or ctx is done.
// On timeout, returns ctx.Err() and increments timeouts.
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

// Release returns one permit. Safe to call exactly once per
// successful Acquire. No-op (returns false) if no permit held.
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

// Run executes op with a permit held for its duration.
// Returns ErrFull if no permit is available.
func (b *Bulkhead) Run(op func() error) error {
	if err := b.Acquire(); err != nil {
		return err
	}
	defer b.Release()
	return op()
}

// RunCtx executes op with a permit acquired with ctx.
func (b *Bulkhead) RunCtx(ctx context.Context, op func() error) error {
	if err := b.AcquireCtx(ctx); err != nil {
		return err
	}
	defer b.Release()
	return op()
}

// Stats is the bulkhead state.
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

// Stats returns a snapshot.
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

// Max returns the configured permit count.
func (b *Bulkhead) Max() int { return b.max }

// Name returns the bulkhead name.
func (b *Bulkhead) Name() string { return b.name }
