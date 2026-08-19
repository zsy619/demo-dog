// Package windowx 提供一个滑动窗口计数器（如限流 QPS 统计）。
package windowx

import "sync"

// Window 是一个 size 秒的滑动窗口，每秒一个格子。
type Window struct {
	mu    sync.Mutex
	size  int
	cells []int64
	now   int
}

// New 创建 size 个格的滑动窗口。
func New(size int) *Window {
	if size < 1 {
		size = 10
	}
	return &Window{size: size, cells: make([]int64, size)}
}

// Add 在当前格子增加 n。
func (w *Window) Add(n int64) {
	w.mu.Lock()
	w.cells[w.now] += n
	w.mu.Unlock()
}

// Tick 推进一格。
func (w *Window) Tick() {
	w.mu.Lock()
	w.now = (w.now + 1) % w.size
	w.cells[w.now] = 0
	w.mu.Unlock()
}

// Sum 返回窗口内总和。
func (w *Window) Sum() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	var s int64
	for _, c := range w.cells {
		s += c
	}
	return s
}

// Reset 清空窗口。
func (w *Window) Reset() {
	w.mu.Lock()
	for i := range w.cells {
		w.cells[i] = 0
	}
	w.mu.Unlock()
}

// Len 返回格子数。
func (w *Window) Len() int { return w.size }
