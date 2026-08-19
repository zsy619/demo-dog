// Package barrier 提供一个并发栅栏：等待 N 个协程全部到达后放行。
package barrier

import "sync"

// Barrier 是带计数的栅栏。
type Barrier struct {
	mu      sync.Mutex
	cond    *sync.Cond
	n       int
	waiting int
	release chan struct{}
}

// New 创建一个容量 n 的栅栏。
func New(n int) *Barrier {
	if n < 1 {
		n = 1
	}
	b := &Barrier{n: n}
	b.cond = sync.NewCond(&b.mu)
	b.release = make(chan struct{}, 1)
	return b
}

// Wait 阻塞直到 n 个并发调用都已 Wait 或 ctx 取消。
func (b *Barrier) Wait() {
	b.mu.Lock()
	b.waiting++
	if b.waiting >= b.n {
		// 放行
		select {
		case b.release <- struct{}{}:
		default:
		}
		b.cond.Broadcast()
		b.mu.Unlock()
		return
	}
	b.cond.Wait()
	b.mu.Unlock()
}

// Release 阻塞等待放行。
func (b *Barrier) Release() {
	<-b.release
}

// Reset 重置栅栏状态。
func (b *Barrier) Reset() {
	b.mu.Lock()
	b.waiting = 0
	select {
	case <-b.release:
	default:
	}
	b.mu.Unlock()
}

// Waiting 返回当前等待计数。
func (b *Barrier) Waiting() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.waiting
}
