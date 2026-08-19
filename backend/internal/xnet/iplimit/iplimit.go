// Package iplimit 提供按客户端 IP 的滑动窗口限流。
package iplimit

import (
	"sync"
	"time"
)

// Limiter 是按 IP 的滑动窗口限流器。
type Limiter struct {
	mu       sync.Mutex
	window   time.Duration
	limit    int
	hits     map[string][]time.Time
	stop     chan struct{}
}

// New 创建一个窗口 window、限额 limit 的限流器。
func New(window time.Duration, limit int) *Limiter {
	if limit <= 0 {
		limit = 100
	}
	l := &Limiter{window: window, limit: limit, hits: make(map[string][]time.Time), stop: make(chan struct{})}
	go l.gc()
	return l
}

// Allow 检查某 IP 的请求是否被允许。
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	hits := l.hits[ip]
	// 移除窗口外
	i := 0
	for ; i < len(hits); i++ {
		if hits[i].After(cutoff) {
			break
		}
	}
	if i > 0 {
		hits = hits[i:]
	}
	if len(hits) >= l.limit {
		l.hits[ip] = hits
		return false
	}
	hits = append(hits, now)
	l.hits[ip] = hits
	return true
}

// Reset 清除某个 IP 的计数。
func (l *Limiter) Reset(ip string) {
	l.mu.Lock()
	delete(l.hits, ip)
	l.mu.Unlock()
}

// Count 返回 IP 当前窗口内的请求数。
func (l *Limiter) Count(ip string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	hits := l.hits[ip]
	n := 0
	for _, h := range hits {
		if h.After(cutoff) {
			n++
		}
	}
	return n
}

// Close 停止后台 GC。
func (l *Limiter) Close() {
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
}

func (l *Limiter) gc() {
	t := time.NewTicker(l.window)
	if l.window > time.Minute {
		t = time.NewTicker(time.Minute)
	}
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			l.purge()
		}
	}
}

func (l *Limiter) purge() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	for ip, hits := range l.hits {
		i := 0
		for ; i < len(hits); i++ {
			if hits[i].After(cutoff) {
				break
			}
		}
		hits = hits[i:]
		if len(hits) == 0 {
			delete(l.hits, ip)
		} else {
			l.hits[ip] = hits
		}
	}
}
