// Package bitset 提供一个简单的位图集合实现，
// 用于成员判定、排序去重与布尔运算。
package bitset

import "sync"

// Bitset 是一个线程安全的位图。
type Bitset struct {
	mu   sync.RWMutex
	bits []uint64
}

// New 返回一个空 Bitset。
func New() *Bitset { return &Bitset{} }

// Set 把第 i 位设为 1。
func (b *Bitset) Set(i uint) {
	b.mu.Lock()
	b.grow(i)
	b.bits[i/64] |= 1 << (i % 64)
	b.mu.Unlock()
}

// Clear 把第 i 位设为 0。
func (b *Bitset) Clear(i uint) {
	b.mu.Lock()
	if int(i/64) < len(b.bits) {
		b.bits[i/64] &^= 1 << (i % 64)
	}
	b.mu.Unlock()
}

// Test 返回第 i 位是否为 1。
func (b *Bitset) Test(i uint) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if int(i/64) >= len(b.bits) {
		return false
	}
	return b.bits[i/64]&(1<<(i%64)) != 0
}

// Count 返回已设置的位数。
func (b *Bitset) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, w := range b.bits {
		for w != 0 {
			w &= w - 1
			n++
		}
	}
	return n
}

// Union 与 o 的并集（修改 b）。
func (b *Bitset) Union(o *Bitset) {
	b.mu.Lock()
	defer b.mu.Unlock()
	o.mu.RLock()
	defer o.mu.RUnlock()
	if len(o.bits) > len(b.bits) {
		b.grow(uint(len(o.bits)*64 - 1))
	}
	for i := range o.bits {
		b.bits[i] |= o.bits[i]
	}
}

// Intersection 与 o 的交集（修改 b）。
func (b *Bitset) Intersection(o *Bitset) {
	b.mu.Lock()
	defer b.mu.Unlock()
	o.mu.RLock()
	defer o.mu.RUnlock()
	n := len(b.bits)
	if len(o.bits) < n {
		n = len(o.bits)
	}
	for i := 0; i < n; i++ {
		b.bits[i] &= o.bits[i]
	}
	for i := n; i < len(b.bits); i++ {
		b.bits[i] = 0
	}
}

// Indices 返回所有已设置的位下标。
func (b *Bitset) Indices() []uint {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := []uint{}
	for i, w := range b.bits {
		for j := 0; j < 64; j++ {
			if w&(1<<j) != 0 {
				out = append(out, uint(i*64+j))
			}
		}
	}
	return out
}

func (b *Bitset) grow(i uint) {
	need := int(i/64) + 1
	if need > len(b.bits) {
		ext := make([]uint64, need)
		copy(ext, b.bits)
		b.bits = ext
	}
}
