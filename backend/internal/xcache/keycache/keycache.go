// Package keycache 提供一个基于 string->string 的高并发 map + 简单 TTL。
package keycache

import (
	"sync"
	"time"
)

// Cache 是带 TTL 的 KV。
type Cache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]entry
}

type entry struct {
	v  string
	ts time.Time
}

// New 创建一个 TTL 缓存。
func New(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, m: make(map[string]entry)}
}

// Put 写入键值。
func (c *Cache) Put(k, v string) {
	c.mu.Lock()
	c.m[k] = entry{v: v, ts: time.Now()}
	c.mu.Unlock()
}

// Get 读取键值，过期返回 false。
func (c *Cache) Get(k string) (string, bool) {
	c.mu.RLock()
	e, ok := c.m[k]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if c.ttl > 0 && time.Since(e.ts) > c.ttl {
		return "", false
	}
	return e.v, true
}

// Delete 删除键。
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

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.m = make(map[string]entry)
	c.mu.Unlock()
}

// GC 删除所有过期键。
func (c *Cache) GC() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ttl <= 0 {
		return 0
	}
	now := time.Now()
	n := 0
	for k, e := range c.m {
		if now.Sub(e.ts) > c.ttl {
			delete(c.m, k)
			n++
		}
	}
	return n
}
