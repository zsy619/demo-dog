// Package qcache 提供按 query key 缓存函数调用结果。
package qcache

import "sync"

// Loader 加载缓存未命中时的数据。
type Loader[K comparable, V any] func(K) (V, error)

// Cache 泛型查询缓存。
type Cache[K comparable, V any] struct {
	mu     sync.RWMutex
	items  map[K]V
	loader Loader[K, V]
}

// New 创建一个查询缓存。
func New[K comparable, V any](loader Loader[K, V]) *Cache[K, V] {
	return &Cache[K, V]{items: make(map[K]V), loader: loader}
}

// Get 查询，命中返回缓存，否则调用 loader 并写入。
func (c *Cache[K, V]) Get(k K) (V, error) {
	c.mu.RLock()
	v, ok := c.items[k]
	c.mu.RUnlock()
	if ok {
		return v, nil
	}
	v, err := c.loader(k)
	if err != nil {
		var zero V
		return zero, err
	}
	c.mu.Lock()
	c.items[k] = v
	c.mu.Unlock()
	return v, nil
}

// Invalidate 删除一个 key。
func (c *Cache[K, V]) Invalidate(k K) {
	c.mu.Lock()
	delete(c.items, k)
	c.mu.Unlock()
}

// Len 返回元素数。
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Clear 清空。
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	c.items = make(map[K]V)
	c.mu.Unlock()
}
