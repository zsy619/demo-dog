// Package nonce Nonce 防重放：跟踪 nonce 使用情况。
package nonce

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Store 保留一个 sliding-window 记录已见的 nonces，
// 用于防止 token 重放攻击。
type Store struct {
	mu       sync.RWMutex
	seen     map[string]time.Time
	ttl      time.Duration
	maxItems int
	now      func() time.Time
}

// ErrReplay 在 nonce 已被使用过的情况下返回。
var ErrReplay = errors.New("nonce replay")

// New 创建一个 Store，带 ttl 与可选的 maxItems 上限。
func New(ttl time.Duration, maxItems int) *Store {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Store{
		seen:     make(map[string]time.Time),
		ttl:      ttl,
		maxItems: maxItems,
		now:      time.Now,
	}
}

// WithTime 覆盖用于测试的时间源。
func (s *Store) WithTime(now func() time.Time) *Store {
	s.now = now
	return s
}

// Check returns ErrReplay if (tenant, nonce, ts) has been
// seen before within the TTL window. Otherwise it records
// the nonce and returns nil.
func (s *Store) Check(tenant, nonce string, ts time.Time) error {
	if nonce == "" {
		return errors.New("empty nonce")
	}
	key := hashKey(tenant, nonce)
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if exp, ok := s.seen[key]; ok {
		if now.Before(exp) {
			return ErrReplay
		}
	}
	if s.maxItems > 0 && len(s.seen) >= s.maxItems {
		s.evictOneLocked()
	}
	s.seen[key] = now.Add(s.ttl)
	return nil
}

// Forget 从存储中移除一个 nonce。
func (s *Store) Forget(tenant, nonce string) {
	key := hashKey(tenant, nonce)
	s.mu.Lock()
	delete(s.seen, key)
	s.mu.Unlock()
}

// Cleanup removes expired entries. Returns the number of
// entries removed.
func (s *Store) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	n := 0
	for k, exp := range s.seen {
		if now.After(exp) {
			delete(s.seen, k)
			n++
		}
	}
	return n
}

// Len 返回已存储 nonce 的数量。
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.seen)
}

func (s *Store) evictOneLocked() {
	var oldestK string
	var oldest time.Time
	first := true
	for k, v := range s.seen {
		if first || v.Before(oldest) {
			oldestK = k
			oldest = v
			first = false
		}
	}
	if oldestK != "" {
		delete(s.seen, oldestK)
	}
}

func hashKey(tenant, nonce string) string {
	h := sha256.New()
	h.Write([]byte(tenant))
	h.Write([]byte(":"))
	h.Write([]byte(nonce))
	return hex.EncodeToString(h.Sum(nil))
}
