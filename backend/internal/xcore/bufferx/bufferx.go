// Package bufferx 提供一个线程安全的字节 buffer。
package bufferx

import (
	"sync"
)

// Buffer 是一个并发安全的字节 buffer。
type Buffer struct {
	mu sync.Mutex
	d  []byte
}

// New 创建容量 cap 的 Buffer。
func New(cap int) *Buffer {
	if cap < 0 {
		cap = 0
	}
	return &Buffer{d: make([]byte, 0, cap)}
}

// Write 追加字节。
func (b *Buffer) Write(p []byte) {
	b.mu.Lock()
	b.d = append(b.d, p...)
	b.mu.Unlock()
}

// WriteByte 追加单字节。
func (b *Buffer) WriteByte(c byte) {
	b.mu.Lock()
	b.d = append(b.d, c)
	b.mu.Unlock()
}

// Bytes 返回副本。
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, len(b.d))
	copy(out, b.d)
	return out
}

// Len 返回长度。
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.d)
}

// Reset 清空。
func (b *Buffer) Reset() {
	b.mu.Lock()
	b.d = b.d[:0]
	b.mu.Unlock()
}

// Read 读取 n 字节（消费）。
func (b *Buffer) Read(n int) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > len(b.d) {
		n = len(b.d)
	}
	out := make([]byte, n)
	copy(out, b.d[:n])
	b.d = b.d[n:]
	return out
}
