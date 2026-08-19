// Package buffer 提供一个固定容量的字节环形缓冲：
// 写入超出容量时覆盖最早数据。
package buffer

import (
	"errors"
	"sync"
)

// ErrEmpty 在空缓冲读取时返回。
var ErrEmpty = errors.New("buffer: 空")

// Ring 是一个固定容量的字节环形缓冲。
type Ring struct {
	mu   sync.Mutex
	data []byte
	hd   int // 下一个写入位置
	tl   int // 下一个读取位置
	cnt  int
}

// New 创建一个容量为 capacity 的环形缓冲。
func New(capacity int) *Ring {
	if capacity <= 0 {
		capacity = 1024
	}
	return &Ring{data: make([]byte, capacity)}
}

// Write 写入一段字节。
func (r *Ring) Write(p []byte) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range p {
		r.data[r.hd] = c
		r.hd = (r.hd + 1) % len(r.data)
		if r.cnt == len(r.data) {
			r.tl = (r.tl + 1) % len(r.data) // 覆盖最早字节
		} else {
			r.cnt++
		}
	}
	return len(p)
}

// Read 读取最多 n 字节到 dst。
func (r *Ring) Read(dst []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cnt == 0 {
		return 0, ErrEmpty
	}
	n := len(dst)
	if n > r.cnt {
		n = r.cnt
	}
	for i := 0; i < n; i++ {
		dst[i] = r.data[r.tl]
		r.tl = (r.tl + 1) % len(r.data)
	}
	r.cnt -= n
	return n, nil
}

// Len 返回当前字节数。
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cnt
}

// Cap 返回容量。
func (r *Ring) Cap() int { return len(r.data) }

// Clear 清空。
func (r *Ring) Clear() {
	r.mu.Lock()
	r.hd = 0
	r.tl = 0
	r.cnt = 0
	r.mu.Unlock()
}

// Bytes 复制一份当前内容快照。
func (r *Ring) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, r.cnt)
	for i := 0; i < r.cnt; i++ {
		out[i] = r.data[(r.tl+i)%len(r.data)]
	}
	return out
}
