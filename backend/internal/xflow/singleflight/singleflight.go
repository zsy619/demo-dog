// Package singleflight 提供简单的 SingleFlight 语义。
// 同一 key 并发调用会合并为一次实际执行。
package singleflight

import "sync"

// Group 是合并相同 key 调用的容器。
type Group[T any] struct {
	mu sync.Mutex
	m  map[string]*call[T]
}

type call[T any] struct {
	wg  sync.WaitGroup
	val T
	err error
}

// New 创建一个 Group。
func New[T any]() *Group[T] {
	return &Group[T]{m: make(map[string]*call[T])}
}

// Do 在 fn 中执行针对 key 的实际操作。
func (g *Group[T]) Do(key string, fn func() (T, error)) (T, error) {
	g.mu.Lock()
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &call[T]{}
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
