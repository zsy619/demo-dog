// Package tinylfu 提供一个最小可用的 TinyLFU 缓存准入策略。
// 它维护一组 4-bit 计数器 sketch，在 Put 时根据频率裁决是否淘汰已有项。
//
// 算法参考 Caffeine 的 W-TinyLFU 简化版：
//   - 维护一个 sketch 估计访问频次
//   - 容量满时从 LRU 末端淘汰受害者
//   - 若受害者频次 > 0 且未尝试过，移到队首（保留）
//   - 否则驱逐
package tinylfu

import (
	"container/list"
	"hash/fnv"
	"sync"
	"sync/atomic"
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

// ResetAll 同时执行 Reset 和清零。
func (s *Sketch) ResetAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cnt {
		s.cnt[i] = 0
	}
}

// Size 返回 sketch 大小（行数）。
func (s *Sketch) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cnt)
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

	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
	inserts   atomic.Uint64
}

type entry struct {
	key   string
	value any
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
		c.misses.Add(1)
		return nil, false
	}
	c.lru.MoveToFront(el)
	c.sketch.Increment([]byte(key))
	c.hits.Add(1)
	return el.Value.(*entry).value, true
}

// Contains 检查 key 是否存在（不更新频次）。
func (c *Cache) Contains(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[key]
	return ok
}

// Peek 取值但不更新 LRU 顺序或频次。
func (c *Cache) Peek(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
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
	c.inserts.Add(1)
	if c.lru.Len() > c.capacity {
		c.evict()
	}
}

// Delete 删除一个 key；返回是否存在。
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return false
	}
	c.lru.Remove(el)
	delete(c.items, key)
	return true
}

// Len 返回当前条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}

// Cap 返回容量。
func (c *Cache) Cap() int {
	return c.capacity
}

// Stats 返回统计快照。
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	sz := c.lru.Len()
	c.mu.Unlock()
	return Stats{
		Size:      sz,
		Capacity:  c.capacity,
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
		Inserts:   c.inserts.Load(),
	}
}

// Clear 清空缓存（包括 sketch）。
func (c *Cache) Clear() {
	c.mu.Lock()
	c.lru = list.New()
	c.items = make(map[string]*list.Element, c.capacity)
	sketch := c.sketch
	c.since = 0
	c.mu.Unlock()
	sketch.ResetAll()
}

func (c *Cache) evict() {
	tried := make(map[string]bool)
	for c.lru.Len() > c.capacity {
		victim := c.lru.Back()
		if victim == nil {
			return
		}
		cand := victim.Value.(*entry)
		if c.sketch.Estimate([]byte(cand.key)) > 0 && !tried[cand.key] {
			tried[cand.key] = true
			c.lru.MoveToFront(victim)
			continue
		}
		c.lru.Remove(victim)
		delete(c.items, cand.key)
		c.evictions.Add(1)
	}
}
