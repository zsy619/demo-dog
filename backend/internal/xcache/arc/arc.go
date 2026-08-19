// Package arc 提供 Adaptive Replacement Cache (ARC) 的简化实现。
// 它使用 4 个 LRU 列表：T1, T2, B1, B2，根据访问模式动态调整 T1 与 T2 的比例。
package arc

import (
	"container/list"
	"sync"
)

// Cache 是 ARC 缓存。
type Cache struct {
	mu       sync.Mutex
	capacity int
	p        int // T1 的目标大小
	t1, t2   *list.List
	b1, b2   *list.List
	idx      map[string]*list.Element
}

type entry struct {
	key   string
	value any
	inList *list.List
}

// New 创建一个容量为 capacity 的 ARC 缓存。
func New(capacity int) *Cache {
	if capacity < 2 {
		capacity = 2
	}
	return &Cache{
		capacity: capacity,
		p:        0,
		t1:       list.New(),
		t2:       list.New(),
		b1:       list.New(),
		b2:       list.New(),
		idx:      make(map[string]*list.Element, capacity),
	}
}

// Get 读取并把命中项移到 T2 头。
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.idx[key]
	if !ok {
		return nil, false
	}
	if el.Value.(*entry).inList == c.t1 {
		c.t1.Remove(el)
		delete(c.idx, key)
		ent := el.Value.(*entry)
		ent.inList = c.t2
		c.idx[key] = c.t2.PushFront(ent)
	} else {
		c.t2.MoveToFront(el)
	}
	return el.Value.(*entry).value, true
}

// Put 写入键值。
func (c *Cache) Put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.idx[key]; ok {
		ent := el.Value.(*entry)
		ent.value = value
		if el.Value.(*entry).inList == c.t1 {
			c.t1.Remove(el)
			ent.inList = c.t2
		c.idx[key] = c.t2.PushFront(ent)
		} else {
			c.t2.MoveToFront(el)
		}
		return
	}
	ent := &entry{key: key, value: value}
	ent.inList = c.t1
		c.idx[key] = c.t1.PushFront(ent)
	c.replace(false)
}

func (c *Cache) replace(adaptB2 bool) {
	for c.totalLen() > c.capacity {
		if !adaptB2 && c.t1.Len() > 0 && (c.t1.Len() > c.p || c.t2.Len() == 0) {
			v := c.t1.Back()
			if v == nil {
				return
			}
			ent := v.Value.(*entry)
			c.t1.Remove(v)
			delete(c.idx, ent.key)
			if c.b1.Len() >= c.capacity {
				c.b1.Remove(c.b1.Front())
			}
			ent.inList = nil
			c.b1.PushFront(ent)
		} else if c.t2.Len() > 0 {
			v := c.t2.Back()
			if v == nil {
				return
			}
			ent := v.Value.(*entry)
			c.t2.Remove(v)
			delete(c.idx, ent.key)
			if c.b2.Len() >= c.capacity {
				c.b2.Remove(c.b2.Front())
			}
			ent.inList = nil
			c.b2.PushFront(ent)
		} else {
			break
		}
	}
}

func (c *Cache) totalLen() int { return c.t1.Len() + c.t2.Len() }

// Len 返回当前条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalLen()
}

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.t1 = list.New()
	c.t2 = list.New()
	c.b1 = list.New()
	c.b2 = list.New()
	c.idx = make(map[string]*list.Element, c.capacity)
	c.mu.Unlock()
}
