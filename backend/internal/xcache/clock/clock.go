// Package clock 实现 CLOCK（也称二次机会）缓存替换算法。
//
// 每个 entry 有一个 used 标记（"时钟手" reference bit）。
// 淘汰时按 FIFO 顺序扫描：
//   - 若 used=true：清除标记并移到队尾
//   - 若 used=false：驱逐
//
// 时间复杂度：Get/Put 平均 O(1)，最差 O(N)。
// 空间复杂度：O(N)。
package clock

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// Cache 是并发安全的 CLOCK 缓存。
type Cache struct {
	mu    sync.Mutex
	cap   int
	items map[string]*list.Element
	order *list.List

	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
	inserts   atomic.Uint64
}

type entry struct {
	key  string
	val  any
	used bool
}

// Stats 是缓存统计。
type Stats struct {
	Size      int    `json:"size"`
	Capacity  int    `json:"capacity"`
	Hits      uint64 `json:"hits"`
	Misses    uint64 `json:"misses"`
	Evictions uint64 `json:"evictions"`
	Inserts   uint64 `json:"inserts"`
}

// New 创建一个容量为 capacity 的缓存（capacity < 2 视为 2）。
func New(capacity int) *Cache {
	if capacity < 2 {
		capacity = 2
	}
	return &Cache{
		cap:   capacity,
		items: make(map[string]*list.Element, capacity),
		order: list.New(),
	}
}

// Get 读取并设置 used=true（标记最近使用）。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		c.misses.Add(1)
		return nil, false
	}
	ent := el.Value.(*entry)
	ent.used = true
	c.hits.Add(1)
	return ent.val, true
}

// Peek 读取但不修改 used 标记。
func (c *Cache) Peek(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		return nil, false
	}
	return el.Value.(*entry).val, true
}

// Contains 检查 key 是否存在（不更新 used）。
func (c *Cache) Contains(k string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[k]
	return ok
}

// Put 写入键值；已存在则更新值并设 used=true。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		ent := el.Value.(*entry)
		ent.val = v
		ent.used = true
		return
	}
	ent := &entry{key: k, val: v, used: true}
	el := c.order.PushBack(ent)
	c.items[k] = el
	c.inserts.Add(1)
	if c.order.Len() > c.cap {
		c.evict()
	}
}

// Delete 移除一个 key；返回是否存在。
func (c *Cache) Delete(k string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		return false
	}
	c.order.Remove(el)
	delete(c.items, k)
	return true
}

// Len 返回当前元素数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Cap 返回容量。
func (c *Cache) Cap() int {
	return c.cap
}

// Stats 返回统计快照。
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	sz := c.order.Len()
	c.mu.Unlock()
	return Stats{
		Size:      sz,
		Capacity:  c.cap,
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
		Inserts:   c.inserts.Load(),
	}
}

// Clear 清空缓存。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element, c.cap)
	c.order = list.New()
	c.mu.Unlock()
}

func (c *Cache) evict() {
	// 二次扫描：每个元素最多被保留一次。
	for i := 0; i < 2*c.order.Len(); i++ {
		front := c.order.Front()
		if front == nil {
			return
		}
		ent := front.Value.(*entry)
		if ent.used {
			ent.used = false
			c.order.MoveToBack(front)
			continue
		}
		c.order.Remove(front)
		delete(c.items, ent.key)
		c.evictions.Add(1)
		return
	}
}
