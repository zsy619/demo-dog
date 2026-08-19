// Package namedcache 提供按名字隔离的多个独立 LRU 缓存。
package namedcache

import "sync"

// Cache 是单个 LRU。
type Cache struct {
	mu    sync.Mutex
	cap   int
	data  map[string]any
	order []string
}

// NewCache 创建一个容量 cap 的 LRU。
func NewCache(cap int) *Cache {
	if cap < 1 {
		cap = 64
	}
	return &Cache{cap: cap, data: make(map[string]any, cap)}
}

// Put 放入一个值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	if _, ok := c.data[k]; !ok {
		c.order = append(c.order, k)
	}
	c.data[k] = v
	for len(c.order) > c.cap {
		del := c.order[0]
		c.order = c.order[1:]
		delete(c.data, del)
	}
	c.mu.Unlock()
}

// Get 读取一个值。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[k]
	return v, ok
}

// Manager 是多个 Cache 的容器。
type Manager struct {
	mu      sync.RWMutex
	caches  map[string]*Cache
	defCap  int
}

// NewManager 创建一个 Manager。
func NewManager(defCap int) *Manager {
	if defCap < 1 {
		defCap = 256
	}
	return &Manager{caches: make(map[string]*Cache), defCap: defCap}
}

// Get 获取或创建一个命名 Cache。
func (m *Manager) Get(name string) *Cache {
	m.mu.RLock()
	c, ok := m.caches[name]
	m.mu.RUnlock()
	if ok {
		return c
	}
	m.mu.Lock()
	if c, ok = m.caches[name]; ok {
		m.mu.Unlock()
		return c
	}
	c = NewCache(m.defCap)
	m.caches[name] = c
	m.mu.Unlock()
	return c
}

// Names 列出所有 cache 名字。
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.caches))
	for k := range m.caches {
		out = append(out, k)
	}
	return out
}
