// Package ratelimit 速率限制：令牌桶与漏桶，支持按 key 隔离。
package ratelimit

// Token bucket + leaky bucket rate limiter.
//
// Two distinct algorithms:
//
//   - TokenBucket: capacity is the burst; refill is the steady
//     rate. Allow up to (capacity) instantly, then refill at
//     `rate` tokens per second. The classic shape for an API
//     gateway.
//
//   - LeakyBucket: queue of pending requests; drain at a fixed
//     rate. Smooth bursty traffic into a constant stream.
//
// Both are sharded by a key (tenant id, IP, API key id) and
// have a maximum number of shards to bound memory.
//
// All public methods are goroutine-safe.

import (
	"errors"
	"sync"
	"time"
)

// ErrLimited is returned when a request is denied.
var ErrLimited = errors.New("rate limit exceeded")

// Settings configures one limiter.
type Settings struct {
	Capacity     int
	RefillPerSec float64
	LeakPerSec   float64
	MaxShards    int
	Now          func() time.Time
}

func (s *Settings) now() func() time.Time {
	if s.Now == nil {
		return time.Now
	}
	return s.Now
}

func (s *Settings) capacity() int {
	if s.Capacity <= 0 {
		return 100
	}
	return s.Capacity
}

func (s *Settings) refill() float64 {
	if s.RefillPerSec <= 0 {
		return 10
	}
	return s.RefillPerSec
}

func (s *Settings) leak() float64 {
	if s.LeakPerSec <= 0 {
		return 10
	}
	return s.LeakPerSec
}

func (s *Settings) maxShards() int {
	if s.MaxShards <= 0 {
		return 10_000
	}
	return s.MaxShards
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

type leakyBucket struct {
	level   float64
	lastDec time.Time
}

// Limiter is the public type.
type Limiter struct {
	mu       sync.Mutex
	settings Settings
	tb       map[string]*tokenBucket
	lb       map[string]*leakyBucket
}

// New returns a Limiter.
func New(s Settings) *Limiter {
	return &Limiter{
		settings: s,
		tb:       make(map[string]*tokenBucket),
		lb:       make(map[string]*leakyBucket),
	}
}

// AllowTokenBucket returns nil if the key has tokens available,
// else ErrLimited.
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

// AllowLeakyBucket returns nil if the bucket can accept the
// request, else ErrLimited.
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

// Tokens returns the current token count for the key.
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

// Reset clears the bucket for one key.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.tb, key)
	delete(l.lb, key)
}

// Snapshot is the JSON-stable view.
type Snapshot struct {
	Shards    int                `json:"shards"`
	TokenKeys []TokenBucketEntry `json:"token_buckets,omitempty"`
	LeakKeys  []LeakyBucketEntry `json:"leak_buckets,omitempty"`
}

// TokenBucketEntry is one row.
type TokenBucketEntry struct {
	Key    string  `json:"key"`
	Tokens float64 `json:"tokens"`
}

// LeakyBucketEntry is one row.
type LeakyBucketEntry struct {
	Key   string  `json:"key"`
	Level float64 `json:"level"`
}

// Snapshot returns the current state.
func (l *Limiter) Snapshot() Snapshot {
	now := l.settings.now()()
	l.mu.Lock()
	defer l.mu.Unlock()
	out := Snapshot{}
	for k, b := range l.tb {
		elapsed := now.Sub(b.lastFill).Seconds()
		tokens := b.tokens
		if elapsed > 0 {
			tokens = b.tokens + elapsed*l.settings.refill()
			if tokens > float64(l.settings.capacity()) {
				tokens = float64(l.settings.capacity())
			}
		}
		out.TokenKeys = append(out.TokenKeys, TokenBucketEntry{Key: k, Tokens: tokens})
	}
	for k, b := range l.lb {
		elapsed := now.Sub(b.lastDec).Seconds()
		level := b.level
		if elapsed > 0 {
			level = b.level - elapsed*l.settings.leak()
			if level < 0 {
				level = 0
			}
		}
		out.LeakKeys = append(out.LeakKeys, LeakyBucketEntry{Key: k, Level: level})
	}
	out.Shards = len(l.tb) + len(l.lb)
	return out
}

// gcLocked evicts shards that have been idle.
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
