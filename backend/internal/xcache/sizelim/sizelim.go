// Package sizelim 提供按字节大小限制的内存缓存：
// 每个条目按其字节大小计费，超出总量时按 LRU 淘汰。
package sizelim

import (
	"container/list"
	"sync"
)

// Cache 是按大小计费的 LRU。
type Cache struct {
	mu        sync.Mutex
	maxBytes  int64
	cur       int64
	items     map[string]*list.Element
	order     *list.List
}

type entry struct {
	key  string
	val  any
	size int64
}

// New 创建一个最大字节数为 maxBytes 的缓存。
func New(maxBytes int64) *Cache {
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1MB
	}
	return &Cache{
		maxBytes: maxBytes,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get 读取并提升到头。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*entry).val, true
}

// Put 写入键值，size 为计费字节数（0 表示按 1 计）。
func (c *Cache) Put(k string, v any, size int64) {
	if size <= 0 {
		size = 1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		ent := el.Value.(*entry)
		c.cur -= ent.size
		ent.val = v
		ent.size = size
		c.cur += size
		c.order.MoveToFront(el)
	} else {
		el := c.order.PushFront(&entry{key: k, val: v, size: size})
		c.items[k] = el
		c.cur += size
	}
	for c.cur > c.maxBytes {
		back := c.order.Back()
		if back == nil {
			break
		}
		ent := back.Value.(*entry)
		c.order.Remove(back)
		delete(c.items, ent.key)
		c.cur -= ent.size
	}
}

// Size 返回当前已用字节。
func (c *Cache) Size() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
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
	c.items = make(map[string]*list.Element)
	c.order = list.New()
	c.cur = 0
	c.mu.Unlock()
}
