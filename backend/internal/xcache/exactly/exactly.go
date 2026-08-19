// Package exactly 提供 exactly-once 标记：
// 标记一个 key 已处理，后续同 key 的 Mark 调用返回 false。
// 用于去重、消费确认等场景。
package exactly

import (
	"sync"
)

// Marker 是 exactly-once 标记器。
type Marker struct {
	mu sync.Mutex
	d  map[string]struct{}
}

// New 创建 Marker。
func New() *Marker { return &Marker{d: make(map[string]struct{})} }

// Mark 标记 key。返回 true 表示首次标记；false 表示重复。
func (m *Marker) Mark(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.d[key]; ok {
		return false
	}
	m.d[key] = struct{}{}
	return true
}

// Unmark 删除标记，允许再次 Mark 通过。
func (m *Marker) Unmark(key string) {
	m.mu.Lock()
	delete(m.d, key)
	m.mu.Unlock()
}

// IsMarked 判断是否已标记。
func (m *Marker) IsMarked(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.d[key]
	return ok
}

// Len 返回已标记数。
func (m *Marker) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.d)
}

// Clear 清空。
func (m *Marker) Clear() {
	m.mu.Lock()
	m.d = make(map[string]struct{})
	m.mu.Unlock()
}
