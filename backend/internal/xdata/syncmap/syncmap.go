// Package syncmap 提供一个轻量并发 map，泛型。
package syncmap

import "sync"

// Map 是 K->V 的并发 map。
type Map[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// New 创建一个空 Map。
func New[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{m: make(map[K]V)}
}

// Set 设置键值。
func (m *Map[K, V]) Set(k K, v V) {
	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
}

// Get 读取键值。
func (m *Map[K, V]) Get(k K) (V, bool) {
	var zero V
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.m[k]
	if !ok {
		return zero, false
	}
	return v, true
}

// Delete 删除键值。
func (m *Map[K, V]) Delete(k K) {
	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
}

// Len 返回元素数。
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

// Keys 返回所有键的切片。
func (m *Map[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]K, 0, len(m.m))
	for k := range m.m {
		out = append(out, k)
	}
	return out
}

// Has 判断键是否存在。
func (m *Map[K, V]) Has(k K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.m[k]
	return ok
}

// Clear 清空。
func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	m.m = make(map[K]V)
	m.mu.Unlock()
}
