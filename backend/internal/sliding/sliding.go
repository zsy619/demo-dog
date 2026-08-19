// Package sliding 提供一个简单的滑动窗口速率限制器。
// 它使用环形数组存储过去 N 个时间戳，按窗口大小判定是否允许新请求。
package sliding

import (
	"sync"
	"time"
)

// Limiter 是每个键一个窗口实例。
type Limiter struct {
	mu     sync.Mutex
	window time.Duration
	limit  int
	now    func() time.Time
	state  map[string]*bucket
}

type bucket struct {
	times []time.Time
}

// New 创建一个允许在 window 内最多 limit 次调用的限制器。
func New(window time.Duration, limit int) *Limiter {
	if window <= 0 {
		window = time.Second
	}
	if limit <= 0 {
		limit = 1
	}
	return &Limiter{
		window: window,
		limit:  limit,
		now:    time.Now,
		state:  make(map[string]*bucket),
	}
}

// WithTime 注入自定义时间源用于测试。
func (l *Limiter) WithTime(now func() time.Time) *Limiter {
	l.now = now
	return l
}

// Allow 判定 key 是否允许一次调用。
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.state[key]
	if !ok {
		b = &bucket{}
		l.state[key] = b
	}
	cutoff := now.Add(-l.window)
	idx := 0
	for ; idx < len(b.times); idx++ {
		if b.times[idx].After(cutoff) {
			break
		}
	}
	b.times = b.times[idx:]
	if len(b.times) >= l.limit {
		return false
	}
	b.times = append(b.times, now)
	return true
}

// Count 返回 key 在当前窗口内已使用的次数。
func (l *Limiter) Count(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.state[key]
	if !ok {
		return 0
	}
	now := l.now()
	cutoff := now.Add(-l.window)
	idx := 0
	for ; idx < len(b.times); idx++ {
		if b.times[idx].After(cutoff) {
			break
		}
	}
	b.times = b.times[idx:]
	return len(b.times)
}

// Reset 清空 key 的窗口。
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.state, key)
	l.mu.Unlock()
}

// Cleanup 移除所有窗口中没有活跃调用的键。
func (l *Limiter) Cleanup() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-l.window)
	n := 0
	for k, b := range l.state {
		idx := 0
		for ; idx < len(b.times); idx++ {
			if b.times[idx].After(cutoff) {
				break
			}
		}
		if idx == len(b.times) {
			delete(l.state, k)
			n++
		} else {
			b.times = b.times[idx:]
		}
	}
	return n
}
