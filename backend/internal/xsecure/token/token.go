// Package token 提供一次性令牌（One-Time Token）的签发与校验。
// 每个令牌只能在有效期内消费一次。
//
// 内存实现 MemStore 会惰性清理过期条目（在 Exists 时扫描）。
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrUnknown 在令牌不存在或已被消费时返回。
var ErrUnknown = errors.New("token: 令牌不存在")

// ErrExpired 在令牌过期时返回。
var ErrExpired = errors.New("token: 令牌过期")

// ErrEmpty 在 secret 为空时返回。
var ErrEmpty = errors.New("token: secret 不能为空")

// Store 是令牌存储抽象。
type Store interface {
	Save(token string, exp time.Time) error
	Exists(token string) (time.Time, bool)
	Delete(token string) error
}

// MemStore 是一个带惰性 GC 的内存 Store 实现。
type MemStore struct {
	mu      sync.Mutex
	entries map[string]time.Time

	// GC 控制
	sweepEvery atomic.Int64 // 每 N 次 Exists 触发一次 GC
	sweeps     atomic.Int64
}

// NewMemStore 创建一个空内存存储；sweepEvery 为惰性 GC 频率（<=0 禁用）。
func NewMemStore() *MemStore {
	s := &MemStore{entries: make(map[string]time.Time)}
	s.sweepEvery.Store(100)
	return s
}

// SetSweepEvery 设置惰性 GC 频率（每 N 次 Exists 触发一次）。
func (s *MemStore) SetSweepEvery(n int64) {
	if n > 0 {
		s.sweepEvery.Store(n)
	}
}

// Save 保存一个令牌与过期时间。
func (s *MemStore) Save(token string, exp time.Time) error {
	s.mu.Lock()
	s.entries[token] = exp
	s.mu.Unlock()
	return nil
}

// Exists 返回 token 的过期时间；不存在返回 false。
// 已过期但尚未 GC 的条目仍返回 (exp, true)，调用方可据此判断。
// 惰性 GC：每 N 次 Exists 触发一次扫描清理过期条目。
func (s *MemStore) Exists(token string) (time.Time, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	// 惰性 GC
	se := s.sweepEvery.Load()
	if se > 0 {
		n := s.sweeps.Add(1)
		if n%se == 0 {
			for k, v := range s.entries {
				if now.After(v) {
					delete(s.entries, k)
				}
			}
		}
	}
	exp, ok := s.entries[token]
	return exp, ok
}

// Delete 移除 token。
func (s *MemStore) Delete(token string) error {
	s.mu.Lock()
	delete(s.entries, token)
	s.mu.Unlock()
	return nil
}

// Len 返回当前条目数（包括已过期）。
func (s *MemStore) Len() int {
	s.mu.Lock()
	n := len(s.entries)
	s.mu.Unlock()
	return n
}

// Sweep 主动清理过期条目；返回清理数。
func (s *MemStore) Sweep() int {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	cleaned := 0
	for k, v := range s.entries {
		if now.After(v) {
			delete(s.entries, k)
			cleaned++
		}
	}
	return cleaned
}

// Generate 生成一个安全的随机 token（hex）。
func Generate(n int) (string, error) {
	if n < 1 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Issue 签发一个随机 token 并存储；TTL 为有效期。
func Issue(s Store, ttl time.Duration) (string, error) {
	tk, err := Generate(32)
	if err != nil {
		return "", err
	}
	exp := time.Now().Add(ttl)
	if err := s.Save(tk, exp); err != nil {
		return "", err
	}
	return tk, nil
}

// Consume 消费一个 token：成功消费后从存储移除。
// 不存在返回 ErrUnknown；过期返回 ErrExpired。
func Consume(s Store, token string) error {
	exp, ok := s.Exists(token)
	if !ok {
		return ErrUnknown
	}
	if time.Now().After(exp) {
		_ = s.Delete(token)
		return ErrExpired
	}
	return s.Delete(token)
}

// Peek 检查 token 是否存在且未过期，但不消费。
func Peek(s Store, token string) error {
	exp, ok := s.Exists(token)
	if !ok {
		return ErrUnknown
	}
	if time.Now().After(exp) {
		return ErrExpired
	}
	return nil
}
