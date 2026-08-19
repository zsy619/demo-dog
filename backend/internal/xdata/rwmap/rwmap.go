// Package rwmap 提供一个 string -> any 的高并发读写 map。
package rwmap

import "sync"

// Map 是 string->any 并发 map。
type Map struct {
	mu sync.RWMutex
	m  map[string]any
}

// New 创建一个空 Map。
func New() *Map { return &Map{m: make(map[string]any)} }

// Put 设置键值。
func (m *Map) Put(k string, v any) {
	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
}

// Get 读取键值。
func (m *Map) Get(k string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.m[k]
	return v, ok
}

// Delete 删除键。
func (m *Map) Delete(k string) {
	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
}

// Len 返回元素数。
func (m *Map) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

// Keys 返回所有键的副本。
func (m *Map) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.m))
	for k := range m.m {
		out = append(out, k)
	}
	return out
}

// Clear 清空。
func (m *Map) Clear() {
	m.mu.Lock()
	m.m = make(map[string]any)
	m.mu.Unlock()
}
