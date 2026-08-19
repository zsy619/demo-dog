// Package ringcache 实现一个简单的 RingBuffer 缓存（覆盖式）。
package ringcache

import "sync"

// Cache 是一个固定大小的环形缓存，写满后从头覆盖。
type Cache[K comparable, V any] struct {
	mu   sync.RWMutex
	cap  int
	keys []K
	vals map[K]V
	idx  int
}

// New 创建容量 cap 的 ringcache。
func New[K comparable, V any](cap int) *Cache[K, V] {
	if cap < 1 {
		cap = 16
	}
	return &Cache[K, V]{
		cap:  cap,
		keys: make([]K, cap),
		vals: make(map[K]V),
	}
}

// Put 写入键值；已满则覆盖最旧。
func (c *Cache[K, V]) Put(k K, v V) {
	c.mu.Lock()
	if _, ok := c.vals[k]; ok {
		c.vals[k] = v
		c.mu.Unlock()
		return
	}
	// 驱逐最旧
	old := c.keys[c.idx]
	delete(c.vals, old)
	c.keys[c.idx] = k
	c.vals[k] = v
	c.idx = (c.idx + 1) % c.cap
	c.mu.Unlock()
}

// Get 读取键值。
func (c *Cache[K, V]) Get(k K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.vals[k]
	return v, ok
}

// Len 返回元素数。
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.vals)
}

// Cap 返回容量。
func (c *Cache[K, V]) Cap() int { return c.cap }

// Clear 清空。
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	c.keys = make([]K, c.cap)
	c.vals = make(map[K]V)
	c.idx = 0
	c.mu.Unlock()
}
