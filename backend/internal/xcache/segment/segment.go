// Package segment 提供按前缀分段的 KV 缓存。
package segment

import (
	"strings"
	"sync"
)

// Cache 是按前缀分段的 KV。
type Cache struct {
	mu       sync.RWMutex
	segments map[string]map[string]any
	defaultCap int
}

// New 创建一个分段缓存。
func New() *Cache {
	return &Cache{segments: make(map[string]map[string]any), defaultCap: 1024}
}

// Put 在 prefix 下放入 key/value。
func (c *Cache) Put(prefix, key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.segments[prefix]
	if !ok {
		m = make(map[string]any, c.defaultCap)
		c.segments[prefix] = m
	}
	m[key] = v
}

// Get 从 prefix 下读取 key。
func (c *Cache) Get(prefix, key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.segments[prefix]
	if !ok {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

// Delete 删除 prefix 下 key。
func (c *Cache) Delete(prefix, key string) {
	c.mu.Lock()
	m, ok := c.segments[prefix]
	if !ok {
		c.mu.Unlock()
		return
	}
	delete(m, key)
	c.mu.Unlock()
}

// ClearPrefix 清空一个 prefix。
func (c *Cache) ClearPrefix(prefix string) {
	c.mu.Lock()
	delete(c.segments, prefix)
	c.mu.Unlock()
}

// Keys 返回所有 key 给定 prefix。
func (c *Cache) Keys(prefix string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.segments[prefix]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Match 以冒号分隔的段查找。
func (c *Cache) Match(prefix string, parts ...string) (any, bool) {
	key := strings.Join(parts, ":")
	return c.Get(prefix, key)
}
