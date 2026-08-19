// Package freq 提供一个按访问频次记录的缓存视图。
package freq

import "sync"

// Counter 统计键的访问次数。
type Counter struct {
	mu     sync.Mutex
	m      map[string]int64
	topN   int
	topHot map[string]int64
}

// New 创建一个最多保留 topN 个 hot key 的计数器。
func New(topN int) *Counter {
	if topN < 1 {
		topN = 100
	}
	return &Counter{m: make(map[string]int64), topN: topN, topHot: make(map[string]int64)}
}

// Inc 增加键计数。
func (c *Counter) Inc(k string) {
	c.mu.Lock()
	c.m[k]++
	if c.m[k] > c.topHot[k] {
		c.topHot[k] = c.m[k]
	}
	c.rebuild()
	c.mu.Unlock()
}

// Count 读取键计数。
func (c *Counter) Count(k string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[k]
}

// Hot 返回访问最多的 n 个键。
func (c *Counter) Hot(n int) []Stat {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Stat, 0, len(c.m))
	for k, v := range c.m {
		out = append(out, Stat{Key: k, Count: v})
	}
	// 简单冒泡排序（n 小）
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Count > out[i].Count {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	return out
}

// Clear 清空。
func (c *Counter) Clear() {
	c.mu.Lock()
	c.m = make(map[string]int64)
	c.topHot = make(map[string]int64)
	c.mu.Unlock()
}

// Stat 是一条统计记录。
type Stat struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

func (c *Counter) rebuild() {
	// 懒重建：只有当 map 大小超过 topN*2 时重建 topHot
	if len(c.m) < c.topN*2 {
		return
	}
	newHot := make(map[string]int64, c.topN)
	for k, v := range c.m {
		if len(newHot) < c.topN {
			newHot[k] = v
		} else {
			// 找最小
			minK := ""
			minV := int64(1<<62)
			for kk, vv := range newHot {
				if vv < minV {
					minV = vv
					minK = kk
				}
			}
			if v > minV {
				delete(newHot, minK)
				newHot[k] = v
			}
		}
	}
	c.topHot = newHot
}
