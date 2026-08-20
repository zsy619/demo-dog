// Package secretrot 密钥轮换：定期更新密钥并保留历史。
package secretrot

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"
)

// Store is a per-tenant secret store with auto-rotation.
type Store struct {
	mu       sync.RWMutex
	secrets  map[string]*entry
	interval time.Duration
	now      func() time.Time
}

type entry struct {
	value     []byte
	created   time.Time
	expires   time.Time
	rotations int
}

// New creates a Store with the rotation interval.
func New(interval time.Duration) *Store {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Store{
		secrets:  make(map[string]*entry),
		interval: interval,
		now:      time.Now,
	}
}

// WithTime overrides the time source for tests.
func (s *Store) WithTime(now func() time.Time) *Store {
	s.now = now
	return s
}

// ErrNotFound is returned for unknown tenants.
var ErrNotFound = errors.New("secret not found")

// Get returns the current secret for tenant. If the secret
// is expired, it is rotated first.
func (s *Store) Get(tenant string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.secrets[tenant]
	if !ok {
		return nil, ErrNotFound
	}
	if !s.now().Before(e.expires) {
		s.rotateLocked(tenant, e)
	}
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, nil
}

// Set installs or replaces a tenant secret.
func (s *Store) Set(tenant string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[tenant] = &entry{
		value:   append([]byte{}, value...),
		created: s.now(),
		expires: s.now().Add(s.interval),
	}
}

// Rotate forces a tenant secret to be regenerated.
func (s *Store) Rotate(tenant string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.secrets[tenant]
	if !ok {
		return ErrNotFound
	}
	s.rotateLocked(tenant, e)
	return nil
}

// Delete removes a tenant secret.
func (s *Store) Delete(tenant string) {
	s.mu.Lock()
	delete(s.secrets, tenant)
	s.mu.Unlock()
}

// Snapshot returns the rotation metadata for all tenants.
func (s *Store) Snapshot() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]time.Time, len(s.secrets))
	for k, e := range s.secrets {
		out[k] = e.expires
	}
	return out
}

// Rotations returns the number of rotations for tenant.
func (s *Store) Rotations(tenant string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.secrets[tenant]
	if !ok {
		return 0
	}
	return e.rotations
}

func (s *Store) rotateLocked(tenant string, e *entry) {
	buf := make([]byte, 32)
	rand.Read(buf)
	e.value = buf
	e.created = s.now()
	e.expires = s.now().Add(s.interval)
	e.rotations++
}
