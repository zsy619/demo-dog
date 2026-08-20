package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Session is one stored session.
type Session struct {
	ID        string
	Subject   string
	Tenant    string
	Data      map[string]any
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store is a session store with sliding TTL.
type Store struct {
	mu       sync.RWMutex
	sess     map[string]*Session
	ttl      time.Duration
	maxItems int
	now      func() time.Time
}

// ErrNotFound is returned when the requested ID is missing.
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

// WithTime overrides the time source for tests.
func (s *Store) WithTime(now func() time.Time) *Store {
	s.now = now
	return s
}

// Create creates a session for subject within tenant.
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

// Get returns the session and slides the expiry.
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

// Peek returns the session without sliding expiry.
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

// Delete removes a session.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.sess, id)
	s.mu.Unlock()
}

// Set attaches a k/v pair to the session, sliding expiry.
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

// Cleanup expires sessions. Returns the count removed.
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

// Len returns the number of live sessions.
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
