// Package ratelimit 提供基于令牌桶的速率限制：
// capacity 表示突发上限，refill 每 tick 添加 token。
package ratelimit

import (
	"sync"
	"time"
)

// Limiter 是令牌桶限流器。
type Limiter struct {
	mu       sync.Mutex
	capacity float64
	refill   float64 // 每秒填充
	tokens   float64
	last     time.Time
}

// New 创建一个容量为 capacity、每秒填充 refill 个令牌的桶。
func New(capacity, refill float64) *Limiter {
	if capacity <= 0 {
		capacity = 1
	}
	if refill <= 0 {
		refill = capacity
	}
	return &Limiter{
		capacity: capacity,
		refill:   refill,
		tokens:   capacity,
		last:     time.Now(),
	}
}

// Allow 尝试消费 1 个令牌。
func (l *Limiter) Allow() bool { return l.AllowN(1) }

// AllowN 尝试消费 n 个令牌。
func (l *Limiter) AllowN(n float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refillTokens()
	if l.tokens >= n {
		l.tokens -= n
		return true
	}
	return false
}

// Wait 阻塞直到获得 1 个令牌。
func (l *Limiter) Wait() {
	for !l.Allow() {
		time.Sleep(time.Millisecond)
	}
}

func (l *Limiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.refill
		if l.tokens > l.capacity {
			l.tokens = l.capacity
		}
		l.last = now
	}
}

// Tokens 返回当前可用令牌。
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refillTokens()
	return l.tokens
}

// SetRate 动态修改填充速率。
func (l *Limiter) SetRate(r float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill = r
}
