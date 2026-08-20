// Package lru2c 2Q 风格 LRU：通过 A1/A2 两段队列模拟 LRU 近似。
package lru2c

import (
	"container/list"
	"sync"
)

// Cache 是带有二次机会淘汰策略的有界 LRU 缓存。
// 每个条目都有一个 referenced 位；淘汰时按 LRU 顺序查找
// 第一个未引用条目，若全部被引用则给予二次机会（清除该位）。
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*entry[K, V]
	order    *list.List
	hits     int
	misses   int
	evicts   int
}

type entry[K comparable, V any] struct {
	key   K
	value V
	ref   bool
	elem  *list.Element
}

// New 创建一个指定容量的 Cache。
func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity <= 0 {
		capacity = 64
	}
	return &Cache[K, V]{
		capacity: capacity,
		items:    make(map[K]*entry[K, V], capacity),
		order:    list.New(),
	}
}

// Get 返回 key 对应的值。会将条目移到队首并设置 referenced 位。
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		var zero V
		c.misses++
		return zero, false
	}
	e.ref = true
	c.order.MoveToFront(e.elem)
	c.hits++
	return e.value, true
}

// Peek 返回值但不会更新 LRU 顺序。
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Put 插入或更新一个条目。超出容量时进行淘汰。
func (c *Cache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		e.value = value
		e.ref = true
		c.order.MoveToFront(e.elem)
		return
	}
	e := &entry[K, V]{key: key, value: value, ref: true}
	e.elem = c.order.PushFront(e)
	c.items[key] = e
	if c.order.Len() > c.capacity {
		c.evictLocked()
	}
}

// Delete 删除一个 key。
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		c.order.Remove(e.elem)
		delete(c.items, key)
	}
}

// Len 返回当前大小。
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Stats 表示计数器集合。
type Stats struct {
	Hits    int `json:"hits"`
	Misses  int `json:"misses"`
	Evicts  int `json:"evicts"`
	Size    int `json:"size"`
	Cap     int `json:"cap"`
}

// Stats 返回当前快照。
func (c *Cache[K, V]) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{Hits: c.hits, Misses: c.misses, Evicts: c.evicts, Size: c.order.Len(), Cap: c.capacity}
}

func (c *Cache[K, V]) evictLocked() {
	for e := c.order.Back(); e != nil; e = e.Prev() {
		ent := e.Value.(*entry[K, V])
		if !ent.ref {
			c.order.Remove(e)
			delete(c.items, ent.key)
			c.evicts++
			return
		}
		ent.ref = false
	}
	// 若全部条目都被引用，则淘汰队尾（LRU）。
	if e := c.order.Back(); e != nil {
		ent := e.Value.(*entry[K, V])
		c.order.Remove(e)
		delete(c.items, ent.key)
		c.evicts++
	}
}
