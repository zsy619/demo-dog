// Package vclock 向量时钟：为分布式事件分配偏序时间戳，检测因果关系。
package vclock

import (
	"sort"
	"sync"
)

// Clock 是向量时钟：节点到计数器的映射。
type Clock struct {
	mu    sync.RWMutex
	entry map[string]uint64
}

// New 创建一个空 Clock。
func New() *Clock {
	return &Clock{entry: make(map[string]uint64)}
}

// Set 替换节点的计数器。
func (c *Clock) Set(node string, v uint64) {
	c.mu.Lock()
	c.entry[node] = v
	c.mu.Unlock()
}

// Tick 将 self 的计数器递增并返回新
// value.
func (c *Clock) Tick(self string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entry[self]++
	return c.entry[self]
}

// Get 返回节点的计数器，若无则返回 0。
func (c *Clock) Get(node string) uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entry[node]
}

// Update 将 other 合并到 c：取逐元素最大值。
// 返回 resulting "score" for ordering.
func (c *Clock) Update(other *Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	for n, v := range other.entry {
		if cur, ok := c.entry[n]; !ok || v > cur {
			c.entry[n] = v
		}
	}
}

// Compare 返回两个时钟的关系：
// -1 if c < other (c is causally before other),
//  0 if c == other (equal),
//  1 if c > other (c is causally after other),
//  2 if c and other are concurrent.
func (c *Clock) Compare(other *Clock) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	less, greater := false, false
	for n, v := range c.entry {
		o := other.entry[n]
		if v < o {
			less = true
		}
		if v > o {
			greater = true
		}
	}
	for n, v := range other.entry {
		if _, ok := c.entry[n]; !ok && v > 0 {
			less = true
		}
	}
	switch {
	case less && greater:
		return 2
	case less:
		return -1
	case greater:
		return 1
	default:
		return 0
	}
}

// Snapshot 返回时钟状态的稳定副本。
func (c *Clock) Snapshot() map[string]uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]uint64, len(c.entry))
	for k, v := range c.entry {
		out[k] = v
	}
	return out
}

// FromSnapshot 从 map 恢复。
func (c *Clock) FromSnapshot(m map[string]uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entry = make(map[string]uint64, len(m))
	for k, v := range m {
		c.entry[k] = v
	}
}

// Nodes 返回已排序的节点列表。
func (c *Clock) Nodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.entry))
	for n := range c.entry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
