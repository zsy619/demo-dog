// Package lruview 提供一个基于 container/list 的最小 LRU 实现，
// 抽象为 OnEvict 回调，便于嵌入业务层。
package lruview

import (
	"container/list"
	"sync"
)

// LRU 是一个固定容量的 LRU 缓存抽象层。
type LRU[K comparable, V any] struct {
	mu      sync.Mutex
	cap     int
	items   map[K]*list.Element
	order   *list.List
	onEvict func(K, V)
}

type entry[K comparable, V any] struct {
	k K
	v V
}

// New 创建一个 LRU。
func New[K comparable, V any](capacity int, onEvict func(K, V)) *LRU[K, V] {
	if capacity <= 0 {
		capacity = 64
	}
	return &LRU[K, V]{
		cap:     capacity,
		items:   make(map[K]*list.Element, capacity),
		order:   list.New(),
		onEvict: onEvict,
	}
}

// Get 读取并提升到头。
func (l *LRU[K, V]) Get(k K) (V, bool) {
	var zero V
	l.mu.Lock()
	defer l.mu.Unlock()
	el, ok := l.items[k]
	if !ok {
		return zero, false
	}
	l.order.MoveToFront(el)
	return el.Value.(*entry[K, V]).v, true
}

// Put 写入键值。
func (l *LRU[K, V]) Put(k K, v V) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if el, ok := l.items[k]; ok {
		el.Value.(*entry[K, V]).v = v
		l.order.MoveToFront(el)
		return
	}
	el := l.order.PushFront(&entry[K, V]{k: k, v: v})
	l.items[k] = el
	if l.order.Len() > l.cap {
		v := l.order.Back()
		if v != nil {
			ent := v.Value.(*entry[K, V])
			l.order.Remove(v)
			delete(l.items, ent.k)
			if l.onEvict != nil {
				l.onEvict(ent.k, ent.v)
			}
		}
	}
}

// Len 返回当前元素数。
func (l *LRU[K, V]) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.order.Len()
}

// Clear 清空。
func (l *LRU[K, V]) Clear() {
	l.mu.Lock()
	l.items = make(map[K]*list.Element, l.cap)
	l.order = list.New()
	l.mu.Unlock()
}

// Keys 返回所有键的有序副本（最近访问在前）。
func (l *LRU[K, V]) Keys() []K {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]K, 0, l.order.Len())
	for e := l.order.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(*entry[K, V]).k)
	}
	return out
}
