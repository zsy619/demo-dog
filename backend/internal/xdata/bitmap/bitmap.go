// Package bitmap 提供一个简单的位图数据结构。
package bitmap

import "sync"

// Bitmap 是一个位向量，支持并发访问。
type Bitmap struct {
	mu   sync.RWMutex
	bits []uint64
}

// New 创建一个可容纳 size 位的位图。
func New(size int) *Bitmap {
	if size < 1 {
		size = 64
	}
	return &Bitmap{bits: make([]uint64, (size+63)/64)}
}

// Set 设置位置 i 的位为 1。
func (b *Bitmap) Set(i int) {
	b.mu.Lock()
	b.grow(i)
	b.bits[i/64] |= 1 << (i % 64)
	b.mu.Unlock()
}

// Clear 清除位置 i 的位。
func (b *Bitmap) Clear(i int) {
	b.mu.Lock()
	if i/64 >= len(b.bits) {
		b.mu.Unlock()
		return
	}
	b.bits[i/64] &^= 1 << (i % 64)
	b.mu.Unlock()
}

// Get 读取位置 i 的位。
func (b *Bitmap) Get(i int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if i/64 >= len(b.bits) {
		return false
	}
	return b.bits[i/64]&(1<<(i%64)) != 0
}

// Count 返回置位的位数。
func (b *Bitmap) Count() int {
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

// Len 返回位数。
func (b *Bitmap) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.bits) * 64
}

func (b *Bitmap) grow(i int) {
	if i/64 < len(b.bits) {
		return
	}
	extra := i/64 + 1 - len(b.bits)
	b.bits = append(b.bits, make([]uint64, extra)...)
}
