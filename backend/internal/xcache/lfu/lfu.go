// Package lfu 实现简单的 LFU（最不经常使用）缓存。
package lfu

import (
	"container/list"
	"sync"
)

// Cache 是 LFU 缓存。
type Cache struct {
	mu    sync.Mutex
	cap   int
	m     map[string]*list.Element
	minF  int
	freq  map[int]*list.List
}

type entry struct {
	k    string
	v    any
	freq int
}

// New 创建容量 cap 的 LFU。
func New(cap int) *Cache {
	if cap <= 0 {
		cap = 128
	}
	return &Cache{
		cap:  cap,
		m:    make(map[string]*list.Element, cap),
		minF: 0,
		freq: make(map[int]*list.List),
	}
}

// Get 读取并提升 freq。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.m[k]
	if !ok {
		return nil, false
	}
	ent := el.Value.(*entry)
	c.promote(el)
	return ent.v, true
}

func (c *Cache) promote(el *list.Element) {
	ent := el.Value.(*entry)
	oldL := c.freq[ent.freq]
	if oldL != nil {
		oldL.Remove(el)
	}
	ent.freq++
	newL, ok := c.freq[ent.freq]
	if !ok {
		newL = list.New()
		c.freq[ent.freq] = newL
	}
	newL.PushFront(el)
	// 如果 minF 频段空了，提升 minF
	for c.minF < ent.freq {
		l, ok := c.freq[c.minF]
		if !ok || l == nil || l.Len() == 0 {
			c.minF++
		} else {
			break
		}
	}
}

// Put 写入键值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[k]; ok {
		ent := el.Value.(*entry)
		ent.v = v
		c.promote(el)
		return
	}
	if len(c.m) >= c.cap {
		c.evictOne()
	}
	l, ok := c.freq[1]
	if !ok {
		l = list.New()
		c.freq[1] = l
	}
	ent := &entry{k: k, v: v, freq: 1}
	el := l.PushFront(ent)
	c.m[k] = el
	if c.minF == 0 || c.minF > 1 {
		c.minF = 1
	}
}

func (c *Cache) evictOne() {
	l, ok := c.freq[c.minF]
	if !ok || l == nil || l.Len() == 0 {
		// 兜底扫描
		c.minF = 1
		l, ok = c.freq[c.minF]
		if !ok {
			return
		}
	}
	back := l.Back()
	if back == nil {
		return
	}
	ent := back.Value.(*entry)
	l.Remove(back)
	delete(c.m, ent.k)
}

// Len 返回元素数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// Cap 返回容量。
func (c *Cache) Cap() int { return c.cap }

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.m = make(map[string]*list.Element, c.cap)
	c.freq = make(map[int]*list.List)
	c.minF = 0
	c.mu.Unlock()
}
