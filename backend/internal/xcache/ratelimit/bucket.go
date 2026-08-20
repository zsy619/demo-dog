package ratelimit

// bucket.go：Limiter 主体与令牌桶 / 漏桶算法实现。
//
// 内部类型 tokenBucket / leakyBucket 仅在 Limiter 内部使用，
// 不对外暴露。Limiter 的所有公共方法都是 goroutine 安全的。

import (
	"sync"
	"time"
)

// tokenBucket 是令牌桶算法的内部状态。
//
// 每次调用 AllowTokenBucket 时：
//  - 计算自上次填充以来的时间增量；
//  - 累加令牌（不超过 capacity）；
//  - 若剩余令牌 < 1 则拒绝；否则扣减 1 个令牌。
type tokenBucket struct {
	tokens   float64   // 当前令牌数
	lastFill time.Time // 上次填充时间
}

// leakyBucket 是漏桶算法的内部状态。
//
// 每次调用 AllowLeakyBucket 时：
//  - 计算自上次漏出以来的时间；
//  - 减去对应漏出量（不低于 0）；
//  - 若 level >= 1 则拒绝；否则 level += 1。
type leakyBucket struct {
	level   float64   // 当前积压请求数
	lastDec time.Time // 上次漏出时间
}

// Limiter 是按 key 隔离的令牌桶 + 漏桶复合限流器。
//
// 线程安全：所有方法都使用 sync.Mutex 保护内部 map。
type Limiter struct {
	mu       sync.Mutex                 // 保护 tb / lb map
	settings Settings                   // 配置
	tb       map[string]*tokenBucket    // 令牌桶分片
	lb       map[string]*leakyBucket    // 漏桶分片
}

// New 创建一个 Limiter。
func New(s Settings) *Limiter {
	return &Limiter{
		settings: s,
		tb:       make(map[string]*tokenBucket),
		lb:       make(map[string]*leakyBucket),
	}
}

// AllowTokenBucket 在 key 仍有令牌时返回 nil，否则返回 ErrLimited。
//
// 首次调用会创建一个满容量（capacity）的桶；
// 当分片数达到 MaxShards 时拒绝新 key。
func (l *Limiter) AllowTokenBucket(key string) error {
	now := l.settings.now()()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked(now)
	b, ok := l.tb[key]
	if !ok {
		if len(l.tb) >= l.settings.maxShards() {
			return ErrLimited
		}
		b = &tokenBucket{tokens: float64(l.settings.capacity()), lastFill: now}
		l.tb[key] = b
	}
	elapsed := now.Sub(b.lastFill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.settings.refill()
		if b.tokens > float64(l.settings.capacity()) {
			b.tokens = float64(l.settings.capacity())
		}
		b.lastFill = now
	}
	if b.tokens < 1 {
		return ErrLimited
	}
	b.tokens -= 1
	return nil
}

// AllowLeakyBucket 在 key 桶未满时返回 nil，否则返回 ErrLimited。
//
// 首次调用会创建一个空（level=0）的桶；
// 当分片数达到 MaxShards 时拒绝新 key。
func (l *Limiter) AllowLeakyBucket(key string) error {
	now := l.settings.now()()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked(now)
	b, ok := l.lb[key]
	if !ok {
		if len(l.lb) >= l.settings.maxShards() {
			return ErrLimited
		}
		b = &leakyBucket{lastDec: now}
		l.lb[key] = b
	}
	elapsed := now.Sub(b.lastDec).Seconds()
	if elapsed > 0 {
		b.level -= elapsed * l.settings.leak()
		if b.level < 0 {
			b.level = 0
		}
		b.lastDec = now
	}
	if b.level >= 1 {
		return ErrLimited
	}
	b.level += 1
	return nil
}

// Tokens 返回 key 当前令牌数（按当前时刻刷新后的值）。
//
// 不存在的 key 视为满容量。
func (l *Limiter) Tokens(key string) float64 {
	now := l.settings.now()()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.tb[key]
	if !ok {
		return float64(l.settings.capacity())
	}
	elapsed := now.Sub(b.lastFill).Seconds()
	if elapsed <= 0 {
		return b.tokens
	}
	v := b.tokens + elapsed*l.settings.refill()
	if v > float64(l.settings.capacity()) {
		return float64(l.settings.capacity())
	}
	return v
}

// Reset 清除 key 对应的令牌桶与漏桶状态。
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.tb, key)
	delete(l.lb, key)
}

// gcLocked 淘汰空闲超过 1 分钟的分片。
//
// 淘汰条件：
//  - tokenBucket：lastFill 早于 1 分钟前，且 tokens 接近满（capacity - 1）；
//  - leakyBucket：lastDec 早于 1 分钟前，且 level ≈ 0。
//
// 必须在持锁状态下调用。
func (l *Limiter) gcLocked(now time.Time) {
	cutoff := now.Add(-time.Minute)
	for k, b := range l.tb {
		if b.lastFill.Before(cutoff) && b.tokens >= float64(l.settings.capacity())-1 {
			delete(l.tb, k)
		}
	}
	for k, b := range l.lb {
		if b.lastDec.Before(cutoff) && b.level <= 0.001 {
			delete(l.lb, k)
		}
	}
}
