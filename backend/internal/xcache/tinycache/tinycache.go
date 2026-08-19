// Package tinycache 提供极简 string->string 缓存（无过期）。
package tinycache

import "sync"

// Cache 是一个固定容量的字符串 KV。
type Cache struct {
	mu  sync.RWMutex
	m   map[string]string
	cap int
}

// New 创建一个容量为 cap 的 Cache。
func New(cap int) *Cache {
	if cap < 1 {
		cap = 256
	}
	return &Cache{m: make(map[string]string, cap), cap: cap}
}

// Set 设置 key=value。
func (c *Cache) Set(k, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.cap {
		// 简单淘汰：随机删一个
		for kk := range c.m {
			delete(c.m, kk)
			break
		}
	}
	c.m[k] = v
}

// Get 读取值。
func (c *Cache) Get(k string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[k]
	return v, ok
}

// Delete 删除 key。
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

// Clear 清空缓存。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.m = make(map[string]string, c.cap)
	c.mu.Unlock()
}
