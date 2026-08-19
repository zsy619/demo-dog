// Package latch 提供一个简单的闩：CountDownLatch 与 StartLatch。
package latch

import "sync"

// CountDownLatch 等待 N 个事件完成。
type CountDownLatch struct {
	mu sync.Mutex
	c  *sync.Cond
	n  int
}

// NewCountDown 创建计数为 n 的 CountDownLatch。
func NewCountDown(n int) *CountDownLatch {
	c := sync.NewCond(&sync.Mutex{})
	return &CountDownLatch{n: n, c: c}
}

// Wait 阻塞直到计数归零。
func (l *CountDownLatch) Wait() {
	l.c.L.Lock()
	for l.n > 0 {
		l.c.Wait()
	}
	l.c.L.Unlock()
}

// CountDown 把计数减 1。
func (l *CountDownLatch) CountDown() {
	l.c.L.Lock()
	l.n--
	if l.n <= 0 {
		l.c.Broadcast()
	}
	l.c.L.Unlock()
}

// StartLatch 等待一个启动信号。
type StartLatch struct {
	mu  sync.Mutex
	done bool
	ch   chan struct{}
}

// NewStartLatch 创建 StartLatch。
func NewStartLatch() *StartLatch {
	return &StartLatch{ch: make(chan struct{})}
}

// Release 发出启动信号。
func (l *StartLatch) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.done {
		return
	}
	l.done = true
	close(l.ch)
}

// Wait 阻塞直到 Release 被调用。
func (l *StartLatch) Wait() {
	<-l.ch
}
