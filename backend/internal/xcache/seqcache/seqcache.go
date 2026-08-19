// Package seqcache 提供一个按 sequence 顺序淘汰的缓存。
package seqcache

import "sync/atomic"

// Cache 是一个按 sequence 顺序淘汰的 FIFO 缓存。
type Cache struct {
	seq  atomic.Uint64
	cap  int
	data map[uint64]*entry
}

type entry struct {
	k   string
	v   any
	s   uint64
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
	c.data[s] = &entry{k: k, v: v, s: s}
	for len(c.data) > c.cap {
		// 找最小 sequence 驱逐
		min := c.seq.Load()
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
}

// Get 读取键值（按字符串匹配）。
func (c *Cache) Get(k string) (any, bool) {
	for _, e := range c.data {
		if e.k == k {
			return e.v, true
		}
	}
	return nil, false
}

// Len 返回元素数。
func (c *Cache) Len() int { return len(c.data) }

// Clear 清空。
func (c *Cache) Clear() {
	c.data = make(map[uint64]*entry)
}
