// Package opool 对象池：复用对象减少 GC 压力。
package opool

import (
	"sync"
	"sync/atomic"
)

// Pool 是一个类似 sync.Pool 风格、带复用钩子的对象池。
type Pool[T any] struct {
	pool      sync.Pool
	new       func() T
	reset     func(T)
	created   atomic.Uint64
	reused    atomic.Uint64
	discarded atomic.Uint64
}

// New 使用给定的工厂函数和 reset 钩子构造一个 Pool。
// 如果不需要 reset，可传入 nil。
func New[T any](new func() T, reset func(T)) *Pool[T] {
	p := &Pool[T]{new: new, reset: reset}
	p.pool.New = func() any { p.created.Add(1); return new() }
	return p
}

// Get 从池中取出一个对象。
func (p *Pool[T]) Get() T {
	v := p.pool.Get()
	if v != nil {
		p.reused.Add(1)
	}
	return v.(T)
}

// Put 将一个对象归还到池中。
// 如果设置了 reset，则先调用 reset。
func (p *Pool[T]) Put(v T) {
	if p.reset != nil {
		p.reset(v)
	}
	p.pool.Put(v)
}

// Stats 返回计数器快照。
type Stats struct {
	Created   uint64 `json:"created"`
	Reused    uint64 `json:"reused"`
	Discarded uint64 `json:"discarded"`
}

// Stats 返回当前快照。
func (p *Pool[T]) Stats() Stats {
	return Stats{Created: p.created.Load(), Reused: p.reused.Load(), Discarded: p.discarded.Load()}
}

// Discard 将一个对象归还给 GC，并递增 discarded 计数器。
func (p *Pool[T]) Discard() {
	p.discarded.Add(1)
}
