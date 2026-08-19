// Package rwmap 提供一个支持原子替换快照的 map，
// 读路径无锁，写路径通过拷贝替换。
package rwmap

import (
	"sync"
	"sync/atomic"
)

// Map 是一个写少读多的 map：
// 写入时构造新 map，原子替换；读时无锁访问快照。
type Map[K comparable, V any] struct {
	mu     sync.Mutex
	dataA  atomic.Pointer[map[K]V]
}

// New 创建一个空 Map。
func New[K comparable, V any]() *Map[K, V] {
	m := &Map[K, V]{}
	empty := make(map[K]V)
	m.dataA.Store(&empty)
	return m
}

// Get 读取键值（无需锁）。
func (m *Map[K, V]) Get(k K) (V, bool) {
	v, ok := m.current()[k]
	return v, ok
}

// Set 写入键值。
func (m *Map[K, V]) Set(k K, v V) {
	m.mu.Lock()
	cur := m.dataA.Load()
	cp := make(map[K]V, len(*cur)+1)
	for kk, vv := range *cur {
		cp[kk] = vv
	}
	cp[k] = v
	m.dataA.Store(&cp)
	m.mu.Unlock()
}

// Delete 移除一个键。
func (m *Map[K, V]) Delete(k K) {
	m.mu.Lock()
	cur := m.dataA.Load()
	cp := make(map[K]V, len(*cur))
	for kk, vv := range *cur {
		if kk != k {
			cp[kk] = vv
		}
	}
	m.dataA.Store(&cp)
	m.mu.Unlock()
}

// Len 返回元素数。
func (m *Map[K, V]) current() map[K]V { return *m.dataA.Load() }

// Len 返回元素数。
func (m *Map[K, V]) Len() int { return len(m.current()) }

// Range 遍历全部元素，回调返回 false 停止。
func (m *Map[K, V]) Range(fn func(K, V) bool) {
	cur := m.current()
	for k, v := range cur {
		if !fn(k, v) {
			return
		}
	}
}

// Snapshot 返回一个浅拷贝 map。
func (m *Map[K, V]) Snapshot() map[K]V {
	cur := m.current()
	out := make(map[K]V, len(cur))
	for k, v := range cur {
		out[k] = v
	}
	return out
}
