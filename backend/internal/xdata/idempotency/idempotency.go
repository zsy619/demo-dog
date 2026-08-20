// Package idempotency 幂等键管理：检测重放请求并去重。
package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

// Record 存储一条幂等响应。
type Record struct {
	Key        string
	RequestHash string
	Status     int
	Body       []byte
	Header     http.Header
	StoredAt   time.Time
}

// Store 是带 TTL 淘汰的幂等键存储。
type Store struct {
	mu       sync.Mutex
	items    map[string]*Record
	ttl      time.Duration
	maxItems int
	now      func() time.Time
}

// New 以给定 TTL 构造一个 Store。
func New(ttl time.Duration, maxItems int) *Store {
	if maxItems <= 0 {
		maxItems = 8192
	}
	return &Store{
		items:    make(map[string]*Record),
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

// Save persists the response for a key.
func (s *Store) Save(key string, status int, body []byte, hdr http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpired()
	if len(s.items) >= s.maxItems {
		s.evictOne()
	}
	s.items[key] = &Record{
		Key: key, Status: status,
		Body: append([]byte{}, body...),
		Header: hdr.Clone(),
		StoredAt: s.now(),
	}
}

// Lookup returns the stored record for key, or nil.
func (s *Store) Lookup(key string) *Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.items[key]
	if !ok {
		return nil
	}
	if s.now().Sub(r.StoredAt) > s.ttl {
		delete(s.items, key)
		return nil
	}
	return r
}

// Forget 删除一个 key。
func (s *Store) Forget(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}

// Len 返回当前条目数。
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

func (s *Store) evictExpired() {
	now := s.now()
	for k, r := range s.items {
		if now.Sub(r.StoredAt) > s.ttl {
			delete(s.items, k)
		}
	}
}

func (s *Store) evictOne() {
	var oldestKey string
	var oldestAt time.Time
	for k, r := range s.items {
		if oldestKey == "" || r.StoredAt.Before(oldestAt) {
			oldestKey = k
			oldestAt = r.StoredAt
		}
	}
	if oldestKey != "" {
		delete(s.items, oldestKey)
	}
}

// HashRequest 返回请求体的稳定 SHA256。
func HashRequest(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

// ErrKeyMissing 由 Middleware.Lookup 在
// caller did not send an Idempotency-Key header.
var ErrKeyMissing = errors.New("idempotency key missing")

// Middleware 在单个端点上强制幂等性。
type Middleware struct {
	Store *Store
	// RequireKey 拒绝缺少 Idempotency-Key 的请求。
	RequireKey bool
	// MismatchBodyHash 在请求时返回错误
	// body differs from the body that produced the cached
	// response.
	MismatchBodyHash bool
}

// Lookup returns the stored record if the request is
// replayed. If MismatchBodyHash is set and the body hash
// differs, it returns nil (the caller should re-process).
func (m *Middleware) Lookup(r *http.Request) (*Record, error) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		if m.RequireKey {
			return nil, ErrKeyMissing
		}
		return nil, nil
	}
	rec := m.Store.Lookup(key)
	if rec == nil {
		return nil, nil
	}
	if m.MismatchBodyHash {
		buf, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(buf))
		if HashRequest(buf) != rec.RequestHash {
			return nil, nil
		}
	}
	return rec, nil
}
