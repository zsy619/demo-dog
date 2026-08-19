// Package tinylfu 提供一个最小可用的 TinyLFU 缓存准入策略。
// 它维护一组计数器 sketch，在 Put 时根据频率裁决是否淘汰已有项。
package tinylfu

import (
	"container/list"
	"hash/fnv"
	"sync"
)

// Sketch 是一个 4-bit 计数器布隆式过滤器。
type Sketch struct {
	mu   sync.Mutex
	mask uint64
	cnt  []byte
}

// NewSketch 创建一个 size（向上取整为 2 的幂）行 4-bit 计数器。
func NewSketch(size int) *Sketch {
	sz := 1
	for sz < size {
		sz <<= 1
	}
	return &Sketch{mask: uint64(sz - 1), cnt: make([]byte, sz)}
}

func (s *Sketch) hash(i int, k []byte) uint64 {
	h := fnv.New64a()
	h.Write([]byte{byte(i)})
	h.Write(k)
	return h.Sum64() & s.mask
}

// Increment 增加 key 的估计频次并返回增加后的最小值。
func (s *Sketch) Increment(key []byte) byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var min byte = 15
	for j := 0; j < 4; j++ {
		idx := s.hash(j, key)
		if s.cnt[idx] < 15 {
			s.cnt[idx]++
		}
		if s.cnt[idx] < min {
			min = s.cnt[idx]
		}
	}
	return min
}

// Estimate 返回 key 的最小估计值。
func (s *Sketch) Estimate(key []byte) byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var min byte = 15
	for j := 0; j < 4; j++ {
		idx := s.hash(j, key)
		if s.cnt[idx] < min {
			min = s.cnt[idx]
		}
	}
	return min
}

// Reset 把所有计数减半（饱和模拟）。
func (s *Sketch) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cnt {
		s.cnt[i] >>= 1
	}
}

// Cache 是 LRU+TinyLFU 复合缓存。
type Cache struct {
	mu       sync.Mutex
	capacity int
	lru      *list.List
	items    map[string]*list.Element
	sketch   *Sketch
	since    int
	window   int
}

type entry struct {
	key   string
	value any
}

// New 创建一个容量为 capacity 的缓存。
func New(capacity int) *Cache {
	if capacity <= 0 {
		capacity = 64
	}
	c := &Cache{
		capacity: capacity,
		lru:      list.New(),
		items:    make(map[string]*list.Element, capacity),
		sketch:   NewSketch(capacity),
		window:   capacity * 10,
	}
	return c
}

// Get 取值并标记最近使用。
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.lru.MoveToFront(el)
	return el.Value.(*entry).value, true
}

// Put 写入键值。
func (c *Cache) Put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sketch.Increment([]byte(key))
	c.since++
	if c.since >= c.window {
		c.sketch.Reset()
		c.since = 0
	}
	if el, ok := c.items[key]; ok {
		el.Value.(*entry).value = value
		c.lru.MoveToFront(el)
		return
	}
	el := c.lru.PushFront(&entry{key: key, value: value})
	c.items[key] = el
	if c.lru.Len() > c.capacity {
		c.evict()
	}
}

func (c *Cache) evict() {
	for c.lru.Len() > c.capacity {
		victim := c.lru.Back()
		if victim == nil {
			return
		}
		cand := victim.Value.(*entry)
		sketch := c.sketch.Estimate([]byte(cand.key))
		// 简化 TinyLFU：被频繁访问的 victim 保留（这里判 0）
		if sketch > 0 && cand != nil {
			// 把 victim 移到前面，避免反复选到它
			c.lru.MoveToFront(victim)
			// 下一个被选的是更早的
			continue
		}
		c.lru.Remove(victim)
		delete(c.items, cand.key)
	}
}

// Len 返回当前条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}

// Clear 清空缓存。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.lru = list.New()
	c.items = make(map[string]*list.Element, c.capacity)
	c.mu.Unlock()
}
