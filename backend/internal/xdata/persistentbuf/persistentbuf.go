// Package persistentbuf 提供一个 append-only 内存 buffer，可整体导出为字节切片。
package persistentbuf

import "sync"

// Buf 是固定顺序写入的并发 buffer。
type Buf struct {
	mu  sync.Mutex
	d   []byte
}

// New 创建空 Buf。
func New() *Buf { return &Buf{} }

// Append 追加字节。
func (b *Buf) Append(p []byte) {
	b.mu.Lock()
	b.d = append(b.d, p...)
	b.mu.Unlock()
}

// AppendByte 追加单字节。
func (b *Buf) AppendByte(c byte) {
	b.mu.Lock()
	b.d = append(b.d, c)
	b.mu.Unlock()
}

// Bytes 返回当前副本。
func (b *Buf) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, len(b.d))
	copy(out, b.d)
	return out
}

// Len 返回长度。
func (b *Buf) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.d)
}

// Reset 清空。
func (b *Buf) Reset() {
	b.mu.Lock()
	b.d = nil
	b.mu.Unlock()
}

// Read 从头读取 n 字节，返回新切片（不修改原 buffer）。
func (b *Buf) Read(n int) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > len(b.d) {
		n = len(b.d)
	}
	out := make([]byte, n)
	copy(out, b.d[:n])
	return out
}
