// Package coalesce 把短时间内重复的同 key 请求合并为一次实际调用。
package coalesce

import "sync"

// Coalescer 是请求合并器。
type Coalescer[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]*flight[V]
}

type flight[V any] struct {
	wg  sync.WaitGroup
	val V
	err error
}

// New 创建一个合并器。
func New[K comparable, V any]() *Coalescer[K, V] {
	return &Coalescer[K, V]{m: make(map[K]*flight[V])}
}

// Do 执行 key 的 fn；同一 key 在执行中再次调用时，等待上一次完成并复用结果。
func (c *Coalescer[K, V]) Do(key K, fn func() (V, error)) (V, error) {
	c.mu.Lock()
	f, ok := c.m[key]
	if ok {
		c.mu.Unlock()
		f.wg.Wait()
		return f.val, f.err
	}
	f = &flight[V]{}
	f.wg.Add(1)
	c.m[key] = f
	c.mu.Unlock()
	f.val, f.err = fn()
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
	f.wg.Done()
	return f.val, f.err
}

// Inflight 返回当前正在执行的 key 数。
func (c *Coalescer[K, V]) Inflight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}
