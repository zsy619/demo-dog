// Package backp 提供一个带背压感知的字节通道包装。
// 当通道容量将满时写入端被暂停，直到消费追上。
package backp

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrClosed 在关闭后写入时返回。
var ErrClosed = errors.New("backp: 已关闭")

// Channel 是带高水位背压的有界字节队列。
type Channel struct {
	mu       sync.Mutex
	cond     *sync.Cond
	buf      [][]byte
	capacity int
	high     int
	low      int
	written   atomic.Uint64
	read      atomic.Uint64
	dropped   atomic.Uint64
	closed   bool
}

// New 创建一个容量为 capacity、高水位 high、低水位 low 的 Channel。
// 当队列长度达到 high 时，Write 阻塞直到队列降到 low 以下。
func New(capacity, high, low int) *Channel {
	if capacity <= 0 {
		capacity = 1024
	}
	if high <= 0 {
		high = capacity * 3 / 4
	}
	if low <= 0 {
		low = capacity / 4
	}
	c := &Channel{capacity: capacity, high: high, low: low, buf: make([][]byte, 0, capacity)}
	c.cond = sync.NewCond(&c.mu)
	return c
}

// Push 写入一段字节。通道高水位时会阻塞。
func (c *Channel) Push(p []byte) error {
	c.mu.Lock()
	for !c.closed && len(c.buf) >= c.high {
		c.cond.Wait()
	}
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	c.buf = append(c.buf, p)
	c.written.Add(1)
	c.mu.Unlock()
	c.cond.Signal()
	return nil
}

// TryPush 非阻塞写入。失败返回 false。
func (c *Channel) TryPush(p []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	if len(c.buf) >= c.high {
		return false
	}
	c.buf = append(c.buf, p)
	c.written.Add(1)
	return true
}

// Pop 弹出一段字节。低水位时阻塞。
func (c *Channel) Pop() ([]byte, error) {
	c.mu.Lock()
	for !c.closed && len(c.buf) == 0 {
		c.cond.Wait()
	}
	if c.closed && len(c.buf) == 0 {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	out := c.buf[0]
	c.buf = c.buf[1:]
	c.read.Add(1)
	c.mu.Unlock()
	c.cond.Broadcast()
	return out, nil
}

// Len 返回当前队列长度。
func (c *Channel) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.buf)
}

// Close 关闭通道。
func (c *Channel) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.cond.Broadcast()
}

// Stats 是计数器快照。
type Stats struct {
	Written  uint64 `json:"written"`
	Read     uint64 `json:"read"`
	Dropped  uint64 `json:"dropped"`
	Len      int    `json:"len"`
	Capacity int    `json:"capacity"`
	High     int    `json:"high"`
	Low      int    `json:"low"`
}

// Stats 返回计数器快照。
func (c *Channel) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Written:  c.written.Load(),
		Read:     c.read.Load(),
		Dropped:  c.dropped.Load(),
		Len:      len(c.buf),
		Capacity: c.capacity,
		High:     c.high,
		Low:      c.low,
	}
}

// PushTimeout 在 timeout 内尝试写入，超时返回 false。
func (c *Channel) PushTimeout(p []byte, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		c.Push(p)
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
