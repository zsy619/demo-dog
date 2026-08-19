// Package fifo 提供一个 FIFO 缓存策略：先入先出，超出容量时淘汰最老的条目。
package fifo

import (
	"container/list"
	"sync"
)

// Cache 是线程安全的 FIFO 缓存。
type Cache struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
}

type entry struct {
	key   string
	value any
}

// New 创建一个容量为 capacity 的 FIFO 缓存。
func New(capacity int) *Cache {
	if capacity <= 0 {
		capacity = 64
	}
	return &Cache{
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Get 读取并把条目移动到队列末尾（语义与 LRU 不同，这里保持 FIFO：不动）。
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	return el.Value.(*entry).value, true
}

// Put 写入键值。容量满时淘汰最老的条目。
func (c *Cache) Put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*entry).value = value
		return
	}
	el := c.order.PushBack(&entry{key: key, value: value})
	c.items[key] = el
	for c.order.Len() > c.capacity {
		victim := c.order.Front()
		if victim == nil {
			break
		}
		delete(c.items, victim.Value.(*entry).key)
		c.order.Remove(victim)
	}
}

// Delete 删除一个键。
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return false
	}
	delete(c.items, key)
	c.order.Remove(el)
	return true
}

// Len 返回当前条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Capacity 返回容量。
func (c *Cache) Capacity() int { return c.capacity }

// Clear 清空缓存。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element, c.capacity)
	c.order.Init()
	c.mu.Unlock()
}

// Keys 按插入顺序返回所有键。
func (c *Cache) Keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, c.order.Len())
	for e := c.order.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(*entry).key)
	}
	return out
}
