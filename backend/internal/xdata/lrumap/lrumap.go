// Package lrumap 提供泛型并发 LRU 缓存（基于 container/list）。
// 替代之前的 lrukv（string->[]byte 专用），通过类型参数支持任意 K、V。
package lrumap

import (
	"container/list"
	"sync"
)

// Map 是泛型 LRU（K comparable，V any）。
type Map[K comparable, V any] struct {
	mu    sync.Mutex
	cap   int
	ll    *list.List
	index map[K]*list.Element
}

type mapEntry[K comparable, V any] struct {
	k K
	v V
}

// New 创建容量 cap 的 Map。
func New[K comparable, V any](cap int) *Map[K, V] {
	if cap < 1 {
		cap = 64
	}
	return &Map[K, V]{
		cap:   cap,
		ll:    list.New(),
		index: make(map[K]*list.Element),
	}
}

// Put 设置键值。
func (m *Map[K, V]) Put(k K, v V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.index[k]; ok {
		el.Value.(*mapEntry[K, V]).v = v
		m.ll.MoveToFront(el)
		return
	}
	if m.ll.Len() >= m.cap {
		back := m.ll.Back()
		if back != nil {
			delete(m.index, back.Value.(*mapEntry[K, V]).k)
			m.ll.Remove(back)
		}
	}
	el := m.ll.PushFront(&mapEntry[K, V]{k: k, v: v})
	m.index[k] = el
}

// Get 读取键值。
func (m *Map[K, V]) Get(k K) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	el, ok := m.index[k]
	if !ok {
		var zero V
		return zero, false
	}
	m.ll.MoveToFront(el)
	return el.Value.(*mapEntry[K, V]).v, true
}

// Delete 删除键。
func (m *Map[K, V]) Delete(k K) {
	m.mu.Lock()
	if el, ok := m.index[k]; ok {
		m.ll.Remove(el)
		delete(m.index, k)
	}
	m.mu.Unlock()
}

// Len 返回元素数。
func (m *Map[K, V]) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ll.Len()
}

// Keys 按访问顺序返回所有 key。
func (m *Map[K, V]) Keys() []K {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]K, 0, m.ll.Len())
	for e := m.ll.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(*mapEntry[K, V]).k)
	}
	return out
}

// Clear 清空。
func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	m.ll.Init()
	m.index = make(map[K]*list.Element)
	m.mu.Unlock()
}

// Cap 返回缓存容量。
func (m *Map[K, V]) Cap() int { return m.cap }
