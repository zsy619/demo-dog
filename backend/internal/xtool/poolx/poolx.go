// Package poolx 提供基于 sync.Pool 的对象池辅助，自动 Reset 与归还。
package poolx

import "sync"

// Pool 是 sync.Pool 的包装，构造对象时由 New 提供。
type Pool[T any] struct {
	p    sync.Pool
	reset func(T)
}

// New 创建一个对象池；reset 在每次 Get 时被调用。
func New[T any](makeFn func() T, reset func(T)) *Pool[T] {
	return &Pool[T]{
		p:    sync.Pool{New: func() any { return makeFn() }},
		reset: reset,
	}
}

// Get 取出对象（如果池中有，先 reset）。
func (p *Pool[T]) Get() T {
	v := p.p.Get().(T)
	if p.reset != nil {
		p.reset(v)
	}
	return v
}

// Put 归还对象。
func (p *Pool[T]) Put(v T) {
	p.p.Put(v)
}

// Use 在函数内使用对象并自动归还。
func (p *Pool[T]) Use(fn func(v T)) {
	v := p.Get()
	defer p.Put(v)
	fn(v)
}
