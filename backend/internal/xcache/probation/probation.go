// Package probation 提供 W-TinyLFU 风格的二级保护缓存。
package probation

import (
	"container/list"
	"sync"
)

// Cache 是二级保护缓存（并发安全）。
type Cache struct {
	mu      sync.Mutex
	mainCap int
	protCap int
	main    map[string]*list.Element
	prot    map[string]*list.Element
	mainLL  *list.List
	protLL  *list.List
}

type entry struct {
	key string
	val any
}

// New 创建一个二级缓存，mainCap 大小，protected = mainCap/4。
func New(mainCap int) *Cache {
	if mainCap < 4 {
		mainCap = 4
	}
	return &Cache{
		mainCap: mainCap,
		protCap: mainCap / 4,
		main:    make(map[string]*list.Element),
		prot:    make(map[string]*list.Element),
		mainLL:  list.New(),
		protLL:  list.New(),
	}
}

// Put 放入键值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.prot[k]; ok {
		el.Value.(*entry).val = v
		c.protLL.MoveToFront(el)
		return
	}
	if el, ok := c.main[k]; ok {
		el.Value.(*entry).val = v
		c.mainLL.MoveToFront(el)
		return
	}
	if c.mainLL.Len() >= c.mainCap {
		back := c.mainLL.Back()
		if back != nil {
			delete(c.main, back.Value.(*entry).key)
			c.mainLL.Remove(back)
		}
	}
	el := c.mainLL.PushFront(&entry{key: k, val: v})
	c.main[k] = el
}

// Get 读取键值。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.prot[k]; ok {
		c.protLL.MoveToFront(el)
		return el.Value.(*entry).val, true
	}
	if el, ok := c.main[k]; ok {
		e := el.Value.(*entry)
		c.mainLL.Remove(el)
		delete(c.main, k)
		c.admitProtected(e)
		return e.val, true
	}
	return nil, false
}

func (c *Cache) admitProtected(e *entry) {
	if c.protLL.Len() >= c.protCap {
		back := c.protLL.Back()
		if back != nil {
			delete(c.prot, back.Value.(*entry).key)
			c.protLL.Remove(back)
		}
	}
	el := c.protLL.PushFront(e)
	c.prot[e.key] = el
}

// Len 返回元素总数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.main) + len(c.prot)
}

// Clear 清空。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.main = make(map[string]*list.Element)
	c.prot = make(map[string]*list.Element)
	c.mainLL.Init()
	c.protLL.Init()
	c.mu.Unlock()
}
