// Package singleflight 提供泛型 SingleFlight 语义。
// 同一 key 并发调用会合并为一次实际执行；返回值与 key 均为类型参数。
package singleflight

import "sync"

// Group 是合并相同 key 调用的容器。
type Group[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]*call[V]
}

type call[V any] struct {
	wg  sync.WaitGroup
	val V
	err error
}

// New 创建一个 Group。
func New[K comparable, V any]() *Group[K, V] {
	return &Group[K, V]{m: make(map[K]*call[V])}
}

// Do 在 fn 中执行针对 key 的实际操作。
func (g *Group[K, V]) Do(key K, fn func() (V, error)) (V, error) {
	g.mu.Lock()
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &call[V]{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()
	c.val, c.err = fn()
	c.wg.Done()
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
	return c.val, c.err
}

// Forget 删除一个正在执行的 key（强制后续调用重新执行）。
// 用于外部取消场景。
func (g *Group[K, V]) Forget(key K) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}

// Inflight 返回正在执行的 key 数。
func (g *Group[K, V]) Inflight() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.m)
}
