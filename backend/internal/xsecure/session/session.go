// Package session 会话管理：会话存储 + 超时清理。
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Session 是一个已存储的会话。
type Session struct {
	ID        string
	Subject   string
	Tenant    string
	Data      map[string]any
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store 是一个具有滑动 TTL 的会话存储。
type Store struct {
	mu       sync.RWMutex
	sess     map[string]*Session
	ttl      time.Duration
	maxItems int
	now      func() time.Time
}

// ErrNotFound 在请求的 ID 缺失时返回。
var ErrNotFound = errors.New("session not found")

// New creates a Store with ttl and optional maxItems (0 = no
// cap).
func New(ttl time.Duration, maxItems int) *Store {
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}
	return &Store{
		sess:     make(map[string]*Session),
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

// Create 在指定租户内为主题创建一个会话。
func (s *Store) Create(subject, tenant string) (*Session, error) {
	now := s.now()
	sess := &Session{
		ID:        newID(),
		Subject:   subject,
		Tenant:    tenant,
		Data:      make(map[string]any),
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}
	s.mu.Lock()
	if s.maxItems > 0 && len(s.sess) >= s.maxItems {
		s.evictOneLocked()
	}
	s.sess[sess.ID] = sess
	s.mu.Unlock()
	return sess, nil
}

// Get 返回会话并顺延过期时间。
func (s *Store) Get(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sess[id]
	if !ok {
		return nil, ErrNotFound
	}
	if s.now().After(sess.ExpiresAt) {
		delete(s.sess, id)
		return nil, ErrNotFound
	}
	sess.ExpiresAt = s.now().Add(s.ttl)
	s.shallowLocked(sess)
	return sess, nil
}

// Peek 返回该会话而不滑动过期时间。
func (s *Store) Peek(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sess[id]
	if !ok {
		return nil, ErrNotFound
	}
	if s.now().After(sess.ExpiresAt) {
		return nil, ErrNotFound
	}
	return s.shallow(sess), nil
}

// Delete 删除一个会话。
func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.sess, id)
	s.mu.Unlock()
}

// Set 为会话附加一个键值对,并滑动过期时间。
func (s *Store) Set(id, key string, val any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sess[id]
	if !ok || s.now().After(sess.ExpiresAt) {
		return ErrNotFound
	}
	sess.Data[key] = val
	sess.ExpiresAt = s.now().Add(s.ttl)
	return nil
}

// Cleanup 清理过期会话,返回被移除的数量。
func (s *Store) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	n := 0
	for id, sess := range s.sess {
		if now.After(sess.ExpiresAt) {
			delete(s.sess, id)
			n++
		}
	}
	return n
}

// Len 返回活跃会话的数量。
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sess)
}

func (s *Store) evictOneLocked() {
	var oldestID string
	var oldest time.Time
	first := true
	for id, sess := range s.sess {
		if first || sess.CreatedAt.Before(oldest) {
			oldestID = id
			oldest = sess.CreatedAt
			first = false
		}
	}
	if oldestID != "" {
		delete(s.sess, oldestID)
	}
}

func (s *Store) shallowLocked(sess *Session) *Session {
	cp := *sess
	cp.Data = make(map[string]any, len(sess.Data))
	for k, v := range sess.Data {
		cp.Data[k] = v
	}
	return &cp
}

func (s *Store) shallow(sess *Session) *Session {
	cp := *sess
	cp.Data = make(map[string]any, len(sess.Data))
	for k, v := range sess.Data {
		cp.Data[k] = v
	}
	return &cp
}

func newID() string {
	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
