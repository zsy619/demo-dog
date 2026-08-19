// Package window 实现一个固定大小、固定时间窗口的滑动缓存。
// 进入窗口的项按 FIFO 淘汰；超过窗口时间的项视为不存在。
package window

import (
	"container/list"
	"sync"
	"time"
)

// Cache 是一个时间窗口内的内存缓存。
type Cache struct {
	mu       sync.Mutex
	cap      int
	window   time.Duration
	items    map[string]*list.Element
	order    *list.List
}

type entry struct {
	key string
	val any
	at  time.Time
}

// New 创建一个 window 时间内最多 cap 个条目的缓存。
func New(cap int, window time.Duration) *Cache {
	if cap <= 0 {
		cap = 1024
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Cache{
		cap:    cap,
		window: window,
		items:  make(map[string]*list.Element, cap),
		order:  list.New(),
	}
}

// Get 读取并把项移到头。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		return nil, false
	}
	ent := el.Value.(*entry)
	if time.Since(ent.at) > c.window {
		c.order.Remove(el)
		delete(c.items, k)
		return nil, false
	}
	c.order.MoveToFront(el)
	return ent.val, true
}

// Put 写入键值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		ent := el.Value.(*entry)
		ent.val = v
		ent.at = time.Now()
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&entry{key: k, val: v, at: time.Now()})
	c.items[k] = el
	if c.order.Len() > c.cap {
		back := c.order.Back()
		if back != nil {
			ent := back.Value.(*entry)
			c.order.Remove(back)
			delete(c.items, ent.key)
		}
	}
}

// Len 返回条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element, c.cap)
	c.order = list.New()
	c.mu.Unlock()
}

// Sweep 主动移除过期项。
func (c *Cache) Sweep() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-c.window)
	n := 0
	for e := c.order.Back(); e != nil; {
		ent := e.Value.(*entry)
		if ent.at.After(cutoff) {
			break
		}
		prev := e.Prev()
		c.order.Remove(e)
		delete(c.items, ent.key)
		n++
		e = prev
	}
	return n
}
