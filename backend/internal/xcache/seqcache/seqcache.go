// Package seqcache 提供一个按 sequence 顺序淘汰的并发安全缓存。
package seqcache

import (
	"sync"
	"sync/atomic"
)

// Cache 是一个按 sequence 顺序淘汰的 FIFO 缓存（并发安全）。
type Cache struct {
	seq  atomic.Uint64
	cap  int
	mu   sync.Mutex
	data map[uint64]*entry
}

type entry struct {
	k string
	v any
	s uint64
}

// New 创建容量 cap 的顺序缓存。
func New(cap int) *Cache {
	if cap < 1 {
		cap = 64
	}
	return &Cache{cap: cap, data: make(map[uint64]*entry)}
}

// Put 放入键值并分配 sequence。
func (c *Cache) Put(k string, v any) {
	s := c.seq.Add(1)
	c.mu.Lock()
	c.data[s] = &entry{k: k, v: v, s: s}
	for len(c.data) > c.cap {
		// 找最小 sequence 驱逐
		var min uint64
		var minKey uint64
		found := false
		for k, e := range c.data {
			if !found || e.s < min {
				min = e.s
				minKey = k
				found = true
			}
		}
		if found {
			delete(c.data, minKey)
		}
	}
	c.mu.Unlock()
}

// Get 读取键值（按字符串匹配）。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.data {
		if e.k == k {
			return e.v, true
		}
	}
	return nil, false
}

// Len 返回元素数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.data = make(map[uint64]*entry)
	c.mu.Unlock()
}
