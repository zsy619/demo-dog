// Package lru2c 2Q 风格 LRU：通过 A1/A2 两段队列模拟 LRU 近似。
package lru2c

import (
	"container/list"
	"sync"
)

// Cache is a bounded LRU with second-chance eviction. Each
// entry has a referenced bit; eviction evicts the first
// unreferenced entry found in LRU order, granting a second
// chance (clearing the bit) if all are referenced.
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

// New creates a Cache with the given capacity.
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

// Get returns the value for key. Moves the entry to the
// front and sets the referenced bit.
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

// Peek returns the value without updating LRU order.
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

// Put inserts or updates an entry. Evicts if over capacity.
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

// Delete removes a key.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		c.order.Remove(e.elem)
		delete(c.items, key)
	}
}

// Len returns the current size.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Stats returns counters.
type Stats struct {
	Hits    int `json:"hits"`
	Misses  int `json:"misses"`
	Evicts  int `json:"evicts"`
	Size    int `json:"size"`
	Cap     int `json:"cap"`
}

// Stats returns the snapshot.
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
	// If everything was referenced, evict the back (LRU).
	if e := c.order.Back(); e != nil {
		ent := e.Value.(*entry[K, V])
		c.order.Remove(e)
		delete(c.items, ent.key)
		c.evicts++
	}
}
