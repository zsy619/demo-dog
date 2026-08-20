// Package qcache 提供按 query key 缓存函数调用结果。
// 同一 key 在加载过程中会合并请求（singleflight）以避免缓存击穿。
package qcache

import (
	"sync"
)

// Loader 加载缓存未命中时的数据。
type Loader[K comparable, V any] func(K) (V, error)

// Cache 泛型查询缓存。
type Cache[K comparable, V any] struct {
	mu     sync.Mutex
	items  map[K]V
	loader Loader[K, V]
	flights map[K]*flight[K, V]
}

type flight[K comparable, V any] struct {
	done chan struct{}
	val  V
	err  error
}

// New 创建一个查询缓存。
func New[K comparable, V any](loader Loader[K, V]) *Cache[K, V] {
	return &Cache[K, V]{
		items:   make(map[K]V),
		loader:  loader,
		flights: make(map[K]*flight[K, V]),
	}
}

// Get 查询，命中返回缓存，否则调用 loader 并写入。
// 同一 key 的并发请求会复用同一次 loader 调用。
func (c *Cache[K, V]) Get(k K) (V, error) {
	c.mu.Lock()
	if v, ok := c.items[k]; ok {
		c.mu.Unlock()
		return v, nil
	}
	if f, ok := c.flights[k]; ok {
		c.mu.Unlock()
		<-f.done
		return f.val, f.err
	}
	f := &flight[K, V]{done: make(chan struct{})}
	c.flights[k] = f
	c.mu.Unlock()
	v, err := c.loader(k)
	c.mu.Lock()
	f.val, f.err = v, err
	if err == nil {
		c.items[k] = v
	}
	delete(c.flights, k)
	close(f.done)
	c.mu.Unlock()
	return v, err
}

// Invalidate 删除一个 key。
func (c *Cache[K, V]) Invalidate(k K) {
	c.mu.Lock()
	delete(c.items, k)
	c.mu.Unlock()
}

// Len 返回元素数。
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Inflight 返回正在加载的 key 数。
func (c *Cache[K, V]) Inflight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.flights)
}

// Clear 清空。
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	c.items = make(map[K]V)
	c.flights = make(map[K]*flight[K, V])
	c.mu.Unlock()
}
