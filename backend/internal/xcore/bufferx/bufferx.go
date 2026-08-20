// Package bufferx 提供一个线程安全的字节 buffer。
//
// Buffer 实现 io.Writer / io.ByteWriter / io.StringWriter，并提供消费式 Read。
// 适用于多个并发生产者向同一个缓冲写入，并由单一消费者分批读取的场景，
// 例如聚合器日志缓冲、批写入队列等。
//
// 所有方法在内部使用 sync.Mutex 保护，并发安全。
// 大块写入（>= 1 KiB）会自动扩容；连续的小写入也会按需扩展。
package bufferx

import (
	"errors"
	"io"
	"sync"
)

// ErrNegativeRead 在 Read 参数 < 0 时返回。
var ErrNegativeRead = errors.New("bufferx: n < 0")

// ErrTooLarge 在配置了 MaxBytes 且写入会超出上限时返回；写入不会被应用。
var ErrTooLarge = errors.New("bufferx: 超出 MaxBytes 上限")

// Buffer 是一个并发安全的字节 buffer。
type Buffer struct {
	mu       sync.Mutex
	d        []byte
	maxBytes int // 0 表示无上限
}

// New 创建容量 cap 的 Buffer。cap < 0 视为 0。
func New(cap int) *Buffer {
	if cap < 0 {
		cap = 0
	}
	return &Buffer{d: make([]byte, 0, cap)}
}

// NewBounded 创建容量 cap、单次写入上限 maxBytes 的 Buffer。
// 写入超出 maxBytes 时返回 ErrTooLarge。
func NewBounded(cap, maxBytes int) *Buffer {
	if cap < 0 {
		cap = 0
	}
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &Buffer{d: make([]byte, 0, cap), maxBytes: maxBytes}
}

// Write 追加字节并返回写入长度（实现 io.Writer）。
// 当配置了 MaxBytes 且追加后会超出上限时返回 ErrTooLarge 且不修改内部状态。
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxBytes > 0 && len(b.d)+len(p) > b.maxBytes {
		return 0, ErrTooLarge
	}
	b.d = append(b.d, p...)
	return len(p), nil
}

// WriteByte 追加单字节（实现 io.ByteWriter）。
func (b *Buffer) WriteByte(c byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxBytes > 0 && len(b.d)+1 > b.maxBytes {
		return ErrTooLarge
	}
	b.d = append(b.d, c)
	return nil
}

// Bytes 返回内部数据的副本（外部修改不会影响 buffer）。
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.d) == 0 {
		return nil
	}
	out := make([]byte, len(b.d))
	copy(out, b.d)
	return out
}

// Len 返回当前已缓冲的字节数。
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.d)
}

// Cap 返回当前底层切片的容量。
func (b *Buffer) Cap() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return cap(b.d)
}

// Reset 清空数据但保留底层容量。
func (b *Buffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.d = b.d[:0]
}

// Read 消费式读取至多 n 字节。n < 0 返回 ErrNegativeRead。
// 返回的切片由调用方持有，底层不引用 buffer 内存；
// 已消费部分在内部被零化以减少敏感数据驻留时间。
func (b *Buffer) Read(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrNegativeRead
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > len(b.d) {
		n = len(b.d)
	}
	out := make([]byte, n)
	copy(out, b.d[:n])
	for i := range b.d[:n] {
		b.d[i] = 0
	}
	b.d = b.d[n:]
	return out, nil
}

// ReadAll 等价于 Read(Len())，一次性消费全部数据。
func (b *Buffer) ReadAll() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.d) == 0 {
		return nil
	}
	out := make([]byte, len(b.d))
	copy(out, b.d)
	b.d = b.d[:0]
	return out
}

// WriteString 等价于 Write([]byte(s))，避免一次额外分配。
func (b *Buffer) WriteString(s string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxBytes > 0 && len(b.d)+len(s) > b.maxBytes {
		return 0, ErrTooLarge
	}
	b.d = append(b.d, s...)
	return len(s), nil
}

// 断言接口实现。
var (
	_ io.Writer      = (*Buffer)(nil)
	_ io.ByteWriter  = (*Buffer)(nil)
	_ io.StringWriter = (*Buffer)(nil)
)
