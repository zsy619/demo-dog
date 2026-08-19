// Package lru2 提供一个近似 LRU-2 的缓存实现：
// 维护两个 LRU 队列，新条目进入 probation（试用）区，
// 二次访问晋升到 protected 区，从 protected 区淘汰的条目降级。
package lru2

import (
	"container/list"
	"sync"
)

// Cache 是 LRU-2 缓存。
type Cache struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	probation *list.List
	protected *list.List
}

type entry struct {
	key   string
	value any
	inList *list.List
}

// New 创建一个容量为 capacity 的 LRU-2 缓存。
func New(capacity int) *Cache {
	if capacity < 2 {
		capacity = 2
	}
	return &Cache{
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		probation: list.New(),
		protected: list.New(),
	}
}

// Get 读取并把条目移动到 protected 队列头。
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if el.Value.(*entry).inList == c.probation {
		c.probation.Remove(el)
		c.items[key] = c.protected.PushFront(el.Value)
	} else {
		c.protected.MoveToFront(el)
	}
	return el.Value.(*entry).value, true
}

// Put 写入键值。
func (c *Cache) Put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*entry).value = value
		if el.Value.(*entry).inList == c.probation {
			c.probation.Remove(el)
			c.items[key] = c.protected.PushFront(el.Value)
		} else {
			c.protected.MoveToFront(el)
		}
		return
	}
	ent := &entry{key: key, value: value}
	el := c.probation.PushFront(ent)
	c.items[key] = el
	c.enforce()
}

func (c *Cache) enforce() {
	protectedCap := c.capacity * 4 / 5
	if protectedCap < 1 {
		protectedCap = 1
	}
	for c.protected.Len() > protectedCap {
		// 降级
		v := c.protected.Back()
		if v == nil {
			break
		}
		ent := v.Value.(*entry)
		c.protected.Remove(v)
		c.items[ent.key] = c.probation.PushFront(ent)
	}
	for c.totalLen() > c.capacity {
		// 从 probation 淘汰
		v := c.probation.Back()
		if v == nil {
			break
		}
		ent := v.Value.(*entry)
		c.probation.Remove(v)
		delete(c.items, ent.key)
	}
}

func (c *Cache) totalLen() int {
	return c.probation.Len() + c.protected.Len()
}

// Len 返回总条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalLen()
}

// Clear 清空缓存。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element, c.capacity)
	c.probation = list.New()
	c.protected = list.New()
	c.mu.Unlock()
}

// Stats 是缓存统计视图。
type Stats struct {
	Probation  int `json:"probation"`
	Protected  int `json:"protected"`
	Capacity   int `json:"capacity"`
}

// Stats 返回各队列长度。
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{Probation: c.probation.Len(), Protected: c.protected.Len(), Capacity: c.capacity}
}
