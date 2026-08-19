// Package lru2 提供泛型 LRU：基于 container/list。
package lru2

import (
	"container/list"
	"sync"
)

// Cache 是一个泛型 LRU。
type Cache[K comparable, V any] struct {
	mu    sync.Mutex
	cap   int
	e     *list.List
	index map[K]*list.Element
}

type entry[K comparable, V any] struct {
	k K
	v V
}

// New 创建一个容量为 cap 的 LRU。
func New[K comparable, V any](cap int) *Cache[K, V] {
	if cap <= 0 {
		cap = 128
	}
	return &Cache[K, V]{
		cap:   cap,
		e:     list.New(),
		index: make(map[K]*list.Element, cap),
	}
}

// Get 读取并提升。
func (c *Cache[K, V]) Get(k K) (V, bool) {
	var zero V
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[k]; ok {
		c.e.MoveToFront(el)
		return el.Value.(*entry[K, V]).v, true
	}
	return zero, false
}

// Put 写入键值。
func (c *Cache[K, V]) Put(k K, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[k]; ok {
		el.Value.(*entry[K, V]).v = v
		c.e.MoveToFront(el)
		return
	}
	el := c.e.PushFront(&entry[K, V]{k: k, v: v})
	c.index[k] = el
	if c.e.Len() > c.cap {
		back := c.e.Back()
		if back != nil {
			ent := back.Value.(*entry[K, V])
			c.e.Remove(back)
			delete(c.index, ent.k)
		}
	}
}

// Len 返回元素数。
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.e.Len()
}

// Cap 返回容量。
func (c *Cache[K, V]) Cap() int { return c.cap }

// Clear 清空。
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	c.e = list.New()
	c.index = make(map[K]*list.Element, c.cap)
	c.mu.Unlock()
}

// Keys 按访问顺序返回所有键。
func (c *Cache[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]K, 0, c.e.Len())
	for el := c.e.Front(); el != nil; el = el.Next() {
		out = append(out, el.Value.(*entry[K, V]).k)
	}
	return out
}
