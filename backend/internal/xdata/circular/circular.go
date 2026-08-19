// Package circular 提供线程安全的循环缓冲。
package circular

import "sync"

// Buffer 是一个固定容量的环形缓冲。
type Buffer struct {
	mu     sync.Mutex
	data   []any
	head   int
	tail   int
	count  int
	full   bool
}

// New 创建容量 cap 的环形缓冲。
func New(cap int) *Buffer {
	if cap <= 0 {
		cap = 16
	}
	return &Buffer{data: make([]any, cap)}
}

// Push 把元素入队；如果满了则覆盖最老元素。
func (b *Buffer) Push(v any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.full {
		b.data[b.tail] = v
		b.tail = (b.tail + 1) % len(b.data)
		b.head = (b.head + 1) % len(b.data)
		return
	}
	b.data[b.head] = v
	b.head = (b.head + 1) % len(b.data)
	b.count++
	if b.head == b.tail {
		b.full = true
	}
}

// Pop 弹出最老元素；空时返回零值与 false。
func (b *Buffer) Pop() (any, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count == 0 && !b.full {
		return nil, false
	}
	v := b.data[b.tail]
	b.data[b.tail] = nil
	b.tail = (b.tail + 1) % len(b.data)
	b.count--
	b.full = false
	return v, true
}

// Len 返回当前元素数。
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.full {
		return len(b.data)
	}
	return b.count
}

// Cap 返回容量。
func (b *Buffer) Cap() int { return len(b.data) }

// Reset 清空。
func (b *Buffer) Reset() {
	b.mu.Lock()
	for i := range b.data {
		b.data[i] = nil
	}
	b.head = 0
	b.tail = 0
	b.count = 0
	b.full = false
	b.mu.Unlock()
}

// Snapshot 按顺序返回所有元素。
func (b *Buffer) Snapshot() []any {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]any, 0, len(b.data))
	if b.full {
		for i := 0; i < len(b.data); i++ {
			idx := (b.tail + i) % len(b.data)
			out = append(out, b.data[idx])
		}
		return out
	}
	for i := 0; i < b.count; i++ {
		idx := (b.tail + i) % len(b.data)
		out = append(out, b.data[idx])
	}
	return out
}
