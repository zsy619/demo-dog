// Package memox 提供一个简单的键值备忘存储（带 LastAccess）。
package memox

import "sync"

type entry struct {
	v  any
	ts int64
}

// Memo 是一个并发 KV。
type Memo struct {
	mu sync.RWMutex
	m  map[string]entry
}

// New 创建一个空 Memo。
func New() *Memo { return &Memo{m: make(map[string]entry)} }

// Set 写入键值并记录时间戳。
func (m *Memo) Set(k string, v any, ts int64) {
	m.mu.Lock()
	m.m[k] = entry{v: v, ts: ts}
	m.mu.Unlock()
}

// Get 读取键值。
func (m *Memo) Get(k string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.m[k]
	if !ok {
		return nil, false
	}
	return e.v, true
}

// LastAccess 返回给定键的访问时间戳。
func (m *Memo) LastAccess(k string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.m[k]
	if !ok {
		return 0, false
	}
	return e.ts, true
}

// Delete 删除键。
func (m *Memo) Delete(k string) {
	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
}

// Len 返回元素数。
func (m *Memo) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

// Clear 清空。
func (m *Memo) Clear() {
	m.mu.Lock()
	m.m = make(map[string]entry)
	m.mu.Unlock()
}
