// Package segment 提供一个分段缓存：将键空间哈希到 N 个独立 segment，
// 每个 segment 是 LRU，整体并发性能优于单一锁的 LRU。
package segment

import (
	"container/list"
	"hash/fnv"
	"sync"
)

// Cache 是分段缓存。
type Cache struct {
	segments []*segment
	n        int
	mask     uint32
}

type segment struct {
	mu   sync.Mutex
	ll   *list.List
	data map[string]*list.Element
	cap  int
}

type entry struct {
	key string
	val any
}

// New 创建一个 segments 个分段、单段容量为 cap 的缓存。
func New(segments, cap int) *Cache {
	if segments <= 0 {
		segments = 16
	}
	if cap <= 0 {
		cap = 1024
	}
	c := &Cache{n: nextPow2(segments), mask: uint32(nextPow2(segments) - 1)}
	c.segments = make([]*segment, c.n)
	for i := 0; i < c.n; i++ {
		c.segments[i] = &segment{
			ll:   list.New(),
			data: make(map[string]*list.Element, cap),
			cap:  cap,
		}
	}
	return c
}

func nextPow2(n int) int {
	v := 1
	for v < n {
		v <<= 1
	}
	return v
}

func (c *Cache) seg(key string) *segment {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.segments[h.Sum32()&c.mask]
}

// Get 读取并把 key 提升到段头。
func (c *Cache) Get(key string) (any, bool) {
	s := c.seg(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.data[key]
	if !ok {
		return nil, false
	}
	s.ll.MoveToFront(el)
	return el.Value.(*entry).val, true
}

// Put 写入键值。
func (c *Cache) Put(key string, val any) {
	s := c.seg(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.data[key]; ok {
		el.Value.(*entry).val = val
		s.ll.MoveToFront(el)
		return
	}
	el := s.ll.PushFront(&entry{key: key, val: val})
	s.data[key] = el
	for s.ll.Len() > s.cap {
		v := s.ll.Back()
		if v == nil {
			break
		}
		s.ll.Remove(v)
		delete(s.data, v.Value.(*entry).key)
	}
}

// Delete 移除一个 key。
func (c *Cache) Delete(key string) {
	s := c.seg(key)
	s.mu.Lock()
	if el, ok := s.data[key]; ok {
		s.ll.Remove(el)
		delete(s.data, key)
	}
	s.mu.Unlock()
}

// Len 返回所有段的条目总和。
func (c *Cache) Len() int {
	total := 0
	for _, s := range c.segments {
		s.mu.Lock()
		total += s.ll.Len()
		s.mu.Unlock()
	}
	return total
}

// Clear 清空所有段。
func (c *Cache) Clear() {
	for _, s := range c.segments {
		s.mu.Lock()
		s.ll = list.New()
		s.data = make(map[string]*list.Element, s.cap)
		s.mu.Unlock()
	}
}
