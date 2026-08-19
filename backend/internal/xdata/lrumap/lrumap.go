// Package lrumap 提供 string->any 的并发 LRU。
package lrumap

import (
	"container/list"
	"sync"
)

// Map 是 string->any 的 LRU。
type Map struct {
	mu     sync.Mutex
	cap    int
	ll     *list.List
	index  map[string]*list.Element
}

type mapEntry struct {
	k string
	v any
}

// New 创建容量 cap 的 Map。
func New(cap int) *Map {
	if cap < 1 {
		cap = 64
	}
	return &Map{cap: cap, ll: list.New(), index: make(map[string]*list.Element)}
}

// Put 设置键值。
func (m *Map) Put(k string, v any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.index[k]; ok {
		el.Value.(*mapEntry).v = v
		m.ll.MoveToFront(el)
		return
	}
	if m.ll.Len() >= m.cap {
		back := m.ll.Back()
		if back != nil {
			delete(m.index, back.Value.(*mapEntry).k)
			m.ll.Remove(back)
		}
	}
	el := m.ll.PushFront(&mapEntry{k: k, v: v})
	m.index[k] = el
}

// Get 读取键值。
func (m *Map) Get(k string) (any, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	el, ok := m.index[k]
	if !ok {
		return nil, false
	}
	m.ll.MoveToFront(el)
	return el.Value.(*mapEntry).v, true
}

// Delete 删除键。
func (m *Map) Delete(k string) {
	m.mu.Lock()
	if el, ok := m.index[k]; ok {
		m.ll.Remove(el)
		delete(m.index, k)
	}
	m.mu.Unlock()
}

// Len 返回元素数。
func (m *Map) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ll.Len()
}

// Clear 清空。
func (m *Map) Clear() {
	m.mu.Lock()
	m.ll.Init()
	m.index = make(map[string]*list.Element)
	m.mu.Unlock()
}
