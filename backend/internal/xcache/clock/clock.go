// Package clock 实现 CLOCK 缓存算法：
// 使用三段 ring（hot/cold/warm）的伪 LRU 替代方案，
// 适合高命中率、近似 LRU 的场景。
package clock

import (
	"container/list"
	"sync"
)

// Cache 是简化的 CLOCK 缓存。
type Cache struct {
	mu       sync.Mutex
	cap      int
	items    map[string]*list.Element
	order    *list.List
}

type entry struct {
	key  string
	val  any
	used bool // 时钟标记
}

// New 创建一个容量为 capacity 的缓存。
func New(capacity int) *Cache {
	if capacity < 2 {
		capacity = 2
	}
	return &Cache{cap: capacity, items: make(map[string]*list.Element, capacity), order: list.New()}
}

// Get 读取并标记使用位。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		return nil, false
	}
	el.Value.(*entry).used = true
	return el.Value.(*entry).val, true
}

// Put 写入键值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		el.Value.(*entry).val = v
		el.Value.(*entry).used = true
		return
	}
	ent := &entry{key: k, val: v, used: true}
	el := c.order.PushBack(ent)
	c.items[k] = el
	if c.order.Len() > c.cap {
		c.evict()
	}
}

func (c *Cache) evict() {
	// 第二次扫描淘汰
	for i := 0; i < 2*c.order.Len(); i++ {
		front := c.order.Front()
		if front == nil {
			return
		}
		ent := front.Value.(*entry)
		if ent.used {
			ent.used = false
			c.order.MoveToBack(front)
		} else {
			c.order.Remove(front)
			delete(c.items, ent.key)
			return
		}
	}
}

// Len 返回元素数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Cap 返回容量。
func (c *Cache) Cap() int { return c.cap }

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element, c.cap)
	c.order = list.New()
	c.mu.Unlock()
}
