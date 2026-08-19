// Package poolx 提供一个按类型构造 + Reset + 自动放回的对象池，
// 与 sync.Pool 不同：对象不会被 GC 主动释放，且支持有界容量。
package poolx

import (
	"errors"
	"sync"
)

// Factory 构造一个新对象。
type Factory func() any

// Resetter 在对象放回前执行清理。
type Resetter func(any)

// Pool 是通用对象池。
type Pool struct {
	mu       sync.Mutex
	items    []any
	capacity int
	newFn    Factory
	reset    Resetter
}

// New 创建一个容量为 capacity 的 Pool。
func New(capacity int, factory Factory, reset Resetter) *Pool {
	if capacity <= 0 {
		capacity = 32
	}
	if factory == nil {
		factory = func() any { return nil }
	}
	return &Pool{capacity: capacity, newFn: factory, reset: reset}
}

// Get 取出一个对象。
func (p *Pool) Get() any {
	p.mu.Lock()
	if n := len(p.items); n > 0 {
		v := p.items[n-1]
		p.items = p.items[:n-1]
		p.mu.Unlock()
		return v
	}
	p.mu.Unlock()
	return p.newFn()
}

// Put 放回对象。容量满时直接丢弃。
func (p *Pool) Put(v any) error {
	if v == nil {
		return errors.New("poolx: nil")
	}
	if p.reset != nil {
		p.reset(v)
	}
	p.mu.Lock()
	if len(p.items) >= p.capacity {
		p.mu.Unlock()
		return nil
	}
	p.items = append(p.items, v)
	p.mu.Unlock()
	return nil
}

// Len 返回池中对象数。
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.items)
}

// Capacity 返回池容量。
func (p *Pool) Capacity() int { return p.capacity }

// Use 是 Put 的便捷封装，配合 defer 使用。
func (p *Pool) Use(fn func(any)) {
	v := p.Get()
	defer p.Put(v)
	fn(v)
}
