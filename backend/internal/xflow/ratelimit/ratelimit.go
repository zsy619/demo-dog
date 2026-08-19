// Package ratelimit 提供基于令牌桶算法的速率限制。
package ratelimit

import (
	"sync"
	"time"
)

// Limiter 是令牌桶限流器。
type Limiter struct {
	mu       sync.Mutex
	capacity float64
	rate     float64         // tokens per second
	tokens   float64
	last     time.Time
}

// New 创建容量为 capacity、补充速率为 rate token/s 的限流器。
func New(capacity, rate float64) *Limiter {
	if capacity <= 0 {
		capacity = 1
	}
	if rate <= 0 {
		rate = 1
	}
	l := &Limiter{capacity: capacity, rate: rate, tokens: capacity, last: time.Now()}
	return l
}

// Allow 尝试消耗 1 个令牌。
func (l *Limiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 尝试消耗 n 个令牌。
func (l *Limiter) AllowN(n float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	l.tokens += elapsed * l.rate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	l.last = now
	if l.tokens >= n {
		l.tokens -= n
		return true
	}
	return false
}

// Wait 阻塞直到允许 1 个令牌或 ctx 超时。
func (l *Limiter) Wait(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if l.Allow() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Tokens 返回当前剩余令牌数（仅供监控）。
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tokens
}

// Rate 返回每秒补充速率。
func (l *Limiter) Rate() float64 { return l.rate }

// Capacity 返回令牌桶容量。
func (l *Limiter) Capacity() float64 { return l.capacity }
