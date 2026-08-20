// Package nonce Nonce 防重放：跟踪 nonce 使用情况。
package nonce

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Store keeps a sliding-window record of seen nonces to
// prevent token replay attacks.
type Store struct {
	mu       sync.RWMutex
	seen     map[string]time.Time
	ttl      time.Duration
	maxItems int
	now      func() time.Time
}

// ErrReplay is returned when a nonce has been seen before.
var ErrReplay = errors.New("nonce replay")

// New creates a Store with ttl and an optional maxItems cap.
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

// WithTime overrides the time source for tests.
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

// Forget removes a nonce from the store.
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

// Len returns the number of stored nonces.
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
