// Package filter 提供布隆过滤器风格的判定工具，
// 用于在大集合中快速确认一个 key 一定不存在。
package filter

import (
	"hash/fnv"
	"sync"
)

// Filter 是一个简单的 4-bit 计数器布隆式过滤器。
type Filter struct {
	mu   sync.Mutex
	mask uint64
	bits []byte
}

// New 创建一个 size 行（向上取整为 2 的幂）布隆式过滤器。
func New(size int) *Filter {
	sz := 1
	for sz < size {
		sz <<= 1
	}
	return &Filter{mask: uint64(sz - 1), bits: make([]byte, sz)}
}

// Add 把 key 加入过滤器。
func (f *Filter) Add(key []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for j := 0; j < 4; j++ {
		idx := f.hash(j, key)
		if f.bits[idx] < 15 {
			f.bits[idx]++
		}
	}
}

// Contains 返回 key 是否可能存在。
func (f *Filter) Contains(key []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for j := 0; j < 4; j++ {
		if f.bits[f.hash(j, key)] == 0 {
			return false
		}
	}
	return true
}

// Reset 清零所有位。
func (f *Filter) Reset() {
	f.mu.Lock()
	for i := range f.bits {
		f.bits[i] = 0
	}
	f.mu.Unlock()
}

// Len 返回底层数组大小。
func (f *Filter) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.bits)
}

func (f *Filter) hash(i int, k []byte) uint64 {
	h := fnv.New64a()
	h.Write([]byte{byte(i)})
	h.Write(k)
	return h.Sum64() & f.mask
}

// CounterFilter 把 4-bit 计数器作为估计频率。
type CounterFilter struct {
	Filter
}

// Estimate 返回 key 的最小计数估计。
func (c *CounterFilter) Estimate(key []byte) byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	var min byte = 15
	for j := 0; j < 4; j++ {
		v := c.bits[c.hash(j, key)]
		if v < min {
			min = v
		}
	}
	return min
}
