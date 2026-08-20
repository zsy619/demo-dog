// Package opool 对象池：复用对象减少 GC 压力。
package opool

import (
	"sync"
	"sync/atomic"
)

// Pool is a sync.Pool-style object pool with reuse hooks.
type Pool[T any] struct {
	pool      sync.Pool
	new       func() T
	reset     func(T)
	created   atomic.Uint64
	reused    atomic.Uint64
	discarded atomic.Uint64
}

// New constructs a Pool with the given factory and reset
// hook. reset may be nil if no reset is needed.
func New[T any](new func() T, reset func(T)) *Pool[T] {
	p := &Pool[T]{new: new, reset: reset}
	p.pool.New = func() any { p.created.Add(1); return new() }
	return p
}

// Get retrieves an object from the pool.
func (p *Pool[T]) Get() T {
	v := p.pool.Get()
	if v != nil {
		p.reused.Add(1)
	}
	return v.(T)
}

// Put returns an object to the pool. Calls reset first if
// set.
func (p *Pool[T]) Put(v T) {
	if p.reset != nil {
		p.reset(v)
	}
	p.pool.Put(v)
}

// Stats returns counter snapshot.
type Stats struct {
	Created   uint64 `json:"created"`
	Reused    uint64 `json:"reused"`
	Discarded uint64 `json:"discarded"`
}

// Stats returns the snapshot.
func (p *Pool[T]) Stats() Stats {
	return Stats{Created: p.created.Load(), Reused: p.reused.Load(), Discarded: p.discarded.Load()}
}

// Discard returns an object to GC and bumps the discarded
// counter.
func (p *Pool[T]) Discard() {
	p.discarded.Add(1)
}
