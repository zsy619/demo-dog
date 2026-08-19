// Package token 提供一次性令牌（One-Time Token）的签发与校验。
// 每个令牌只能在有效期内消费一次。
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ErrUnknown 在令牌不存在或已被消费时返回。
var ErrUnknown = errors.New("token: 令牌不存在")

// ErrExpired 在令牌过期时返回。
var ErrExpired = errors.New("token: 令牌过期")

// Store 是令牌存储抽象。
type Store interface {
	Save(token string, exp time.Time) error
	Exists(token string) (time.Time, bool)
	Delete(token string) error
}

// MemStore 是一个简单的内存 Store 实现。
type MemStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

// NewMemStore 创建一个空内存存储。
func NewMemStore() *MemStore {
	return &MemStore{entries: make(map[string]time.Time)}
}

// Save 保存一个令牌与过期时间。
func (s *MemStore) Save(token string, exp time.Time) error {
	s.mu.Lock()
	s.entries[token] = exp
	s.mu.Unlock()
	return nil
}

// Exists 返回 token 的过期时间；不存在返回 false。
func (s *MemStore) Exists(token string) (time.Time, bool) {
	s.mu.Lock()
	exp, ok := s.entries[token]
	s.mu.Unlock()
	return exp, ok
}

// Delete 移除 token。
func (s *MemStore) Delete(token string) error {
	s.mu.Lock()
	delete(s.entries, token)
	s.mu.Unlock()
	return nil
}

// Issue 签发一个 32 字节随机令牌并保存。
func Issue(store Store, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b)
	exp := time.Now().Add(ttl)
	if err := store.Save(t, exp); err != nil {
		return "", err
	}
	return t, nil
}

// Consume 校验并消费一个令牌。
func Consume(store Store, token string) error {
	exp, ok := store.Exists(token)
	if !ok {
		return ErrUnknown
	}
	if time.Now().After(exp) {
		store.Delete(token)
		return ErrExpired
	}
	return store.Delete(token)
}

// Peek 检查令牌但不移除。
func Peek(store Store, token string) error {
	exp, ok := store.Exists(token)
	if !ok {
		return ErrUnknown
	}
	if time.Now().After(exp) {
		return ErrExpired
	}
	return nil
}
