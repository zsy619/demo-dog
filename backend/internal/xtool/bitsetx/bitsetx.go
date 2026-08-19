// Package bitsetx 提供一个固定大小的位集。
package bitsetx

import "sync"

// BitSet 是固定大小的位集。
type BitSet struct {
	mu   sync.RWMutex
	bits []uint64
}

// New 创建容量为 n 的位集。
func New(n int) *BitSet {
	if n < 1 {
		n = 64
	}
	return &BitSet{bits: make([]uint64, (n+63)/64)}
}

// Set 设置位置 i 的位。
func (b *BitSet) Set(i int) {
	b.mu.Lock()
	b.ensure(i)
	b.bits[i/64] |= 1 << (i % 64)
	b.mu.Unlock()
}

// Clear 清除位置 i 的位。
func (b *BitSet) Clear(i int) {
	b.mu.Lock()
	if i/64 >= len(b.bits) {
		b.mu.Unlock()
		return
	}
	b.bits[i/64] &^= 1 << (i % 64)
	b.mu.Unlock()
}

// Get 读取位置 i 的位。
func (b *BitSet) Get(i int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if i/64 >= len(b.bits) {
		return false
	}
	return b.bits[i/64]&(1<<(i%64)) != 0
}

// Count 返回置位数。
func (b *BitSet) Count() int {
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

// Len 返回容量。
func (b *BitSet) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.bits) * 64
}

// Or 修改为与 other 的并集。
func (b *BitSet) Or(other *BitSet) {
	b.mu.Lock()
	defer b.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	n := len(b.bits)
	if len(other.bits) < n {
		n = len(other.bits)
	}
	for i := 0; i < n; i++ {
		b.bits[i] |= other.bits[i]
	}
}

// And 修改为与 other 的交集。
func (b *BitSet) And(other *BitSet) {
	b.mu.Lock()
	defer b.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	n := len(b.bits)
	if len(other.bits) < n {
		n = len(other.bits)
	}
	for i := 0; i < n; i++ {
		b.bits[i] &= other.bits[i]
	}
	for i := n; i < len(b.bits); i++ {
		b.bits[i] = 0
	}
}

func (b *BitSet) ensure(i int) {
	if i/64 < len(b.bits) {
		return
	}
	extra := i/64 + 1 - len(b.bits)
	b.bits = append(b.bits, make([]uint64, extra)...)
}
