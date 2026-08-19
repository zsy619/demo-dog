// Package rwcache 提供一个支持统计的简单并发缓存（基于 sync.RWMutex）。
package rwcache

import (
	"sync"
	"sync/atomic"
)

// Cache 是一个简单的并发 KV 缓存。
type Cache struct {
	mu   sync.RWMutex
	m    map[string]any
	hits atomic.Int64
	miss atomic.Int64
}

// New 创建空 Cache。
func New() *Cache {
	return &Cache{m: make(map[string]any)}
}

// Put 写入键值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	c.m[k] = v
	c.mu.Unlock()
}

// Get 读取键值。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.RLock()
	v, ok := c.m[k]
	c.mu.RUnlock()
	if ok {
		c.hits.Add(1)
	} else {
		c.miss.Add(1)
	}
	return v, ok
}

// Delete 删除键值。
func (c *Cache) Delete(k string) {
	c.mu.Lock()
	delete(c.m, k)
	c.mu.Unlock()
}

// Len 返回元素数。
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}

// Hits 返回命中次数。
func (c *Cache) Hits() int64 { return c.hits.Load() }

// Misses 返回未命中次数。
func (c *Cache) Misses() int64 { return c.miss.Load() }

// HitRate 返回命中率（0-1）。
func (c *Cache) HitRate() float64 {
	h := c.hits.Load()
	m := c.miss.Load()
	total := h + m
	if total == 0 {
		return 0
	}
	return float64(h) / float64(total)
}
