// Package clocksweep 实现 ClockSweep 算法：环形 + "used" 标志位。
package clocksweep

import "sync"

// Cache 是 ClockSweep 缓存。
type Cache struct {
	mu    sync.Mutex
	slots []slot
	hand  int
}

type slot struct {
	key   string
	val   any
	used  bool
	valid bool
}

// New 创建容量 cap 的 ClockSweep。
func New(cap int) *Cache {
	if cap < 1 {
		cap = 64
	}
	return &Cache{slots: make([]slot, cap)}
}

// Put 放入键值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.slots {
		if c.slots[i].valid && c.slots[i].key == k {
			c.slots[i].val = v
			c.slots[i].used = true
			return
		}
	}
	// 找一个空位或可替换的位置
	for tries := 0; tries < len(c.slots); tries++ {
		s := &c.slots[c.hand]
		if !s.valid {
			s.key = k
			s.val = v
			s.used = true
			s.valid = true
			c.hand = (c.hand + 1) % len(c.slots)
			return
		}
		if s.used {
			s.used = false
			c.hand = (c.hand + 1) % len(c.slots)
			continue
		}
		// 找到 victim
		s.key = k
		s.val = v
		s.used = true
		s.valid = true
		c.hand = (c.hand + 1) % len(c.slots)
		return
	}
}

// Get 读取键值，并把 used 置位。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.slots {
		if c.slots[i].valid && c.slots[i].key == k {
			c.slots[i].used = true
			return c.slots[i].val, true
		}
	}
	return nil, false
}

// Len 返回元素数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, s := range c.slots {
		if s.valid {
			n++
		}
	}
	return n
}
