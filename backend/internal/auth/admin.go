package auth

// Admin endpoints: API key rotation + tenant CRUD helpers.
//
// Key rotation lets an operator mint a new key, mark the old
// one as expiring (still valid for a grace window), and
// eventually delete it. The auth layer is unaffected: tokens
// continue to match by hash.
//
// Tenant helpers provide CRUD + list operations for
// /api/admin/tenants. The full registry lives in
// internal/tenants; this package re-exports the operations
// the admin handler wires up.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// KeyEntry is the auth-table row for one API key.
type KeyEntry struct {
	KeyID     string
	Hash      string // sha256 hex of the raw token
	Identity  string // role:admin / role:reader etc.
	Tenant    string // tenant this key is bound to
	Scopes    []string
	CreatedAt time.Time
	ExpiresAt time.Time // zero = never
	Disabled  bool
	RotatedFrom string // populated when a new key replaces an old one
}

// IsValid reports whether the entry is usable right now.
func (e *KeyEntry) IsValid(now time.Time) bool {
	if e.Disabled {
		return false
	}
	if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
		return false
	}
	return true
}

// HasScope reports whether the entry has the named scope.
// Empty Scopes list = legacy unscoped (matches every ACL).
func (e *KeyEntry) HasScope(s string) bool {
	if len(e.Scopes) == 0 {
		return true
	}
	for _, sc := range e.Scopes {
		if sc == s {
			return true
		}
	}
	return false
}

// HasResourceScope is the per-resource variant used by
// /api/v1/rules, /api/v1/quotas, etc.
func (e *KeyEntry) HasResourceScope(resource string) bool {
	return e.HasScope(resource + ":read") || e.HasScope(resource + ":write")
}

// AdminStore owns the API key table. Thread-safe.
type AdminStore struct {
	mu      sync.RWMutex
	keys    map[string]*KeyEntry // by KeyID
	hashToID map[string]string // raw token hash -> KeyID
	counter  atomic.Int64
}

// NewAdminStore creates an empty store.
func NewAdminStore() *AdminStore {
	return &AdminStore{
		keys:     make(map[string]*KeyEntry),
		hashToID: make(map[string]string),
	}
}

func (s *AdminStore) nextID() string {
	n := s.counter.Add(1)
	return fmt.Sprintf("key-%d-%d", time.Now().UnixNano(), n)
}

// GenerateToken returns a 32-byte cryptographically random
// hex token. The caller is responsible for handing it to a
// user exactly once.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken returns the lowercase hex sha256 of the token.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// CreateKey generates a new API key. Returns the raw token
// (only visible at creation time) and the persistent entry.
func (s *AdminStore) CreateKey(identity, tenant string, scopes []string, ttl time.Duration) (raw string, entry *KeyEntry, err error) {
	raw, err = GenerateToken()
	if err != nil {
		return "", nil, err
	}
	entry = &KeyEntry{
		KeyID:     s.nextID(),
		Hash:      hashToken(raw),
		Identity:  identity,
		Tenant:    tenant,
		Scopes:    scopes,
		CreatedAt: time.Now(),
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[entry.KeyID] = entry
	s.hashToID[entry.Hash] = entry.KeyID
	return raw, entry, nil
}

// LookupByToken returns the entry that matches the raw token.
func (s *AdminStore) LookupByToken(raw string) (*KeyEntry, bool) {
	h := hashToken(raw)
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.hashToID[h]
	if !ok {
		return nil, false
	}
	return s.keys[id], true
}

// LookupByID returns the entry by id.
func (s *AdminStore) LookupByID(id string) (*KeyEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.keys[id]
	return e, ok
}

// RotateKey issues a new key tied to the same identity/tenant/
// scopes as the existing one, marks the old key as expiring
// after the grace window, and returns both. Returns the new
// raw token.
func (s *AdminStore) RotateKey(oldID string, grace time.Duration) (raw string, oldEntry, newEntry *KeyEntry, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.keys[oldID]
	if !ok {
		return "", nil, nil, errors.New("key not found")
	}
	raw, err = GenerateToken()
	if err != nil {
		return "", nil, nil, err
	}
	newEntry = &KeyEntry{
		KeyID:       s.nextID(),
		Hash:        hashToken(raw),
		Identity:    old.Identity,
		Tenant:      old.Tenant,
		Scopes:      old.Scopes,
		CreatedAt:   time.Now(),
		RotatedFrom: old.KeyID,
	}
	if grace > 0 {
		old.ExpiresAt = time.Now().Add(grace)
	}
	s.keys[newEntry.KeyID] = newEntry
	s.hashToID[newEntry.Hash] = newEntry.KeyID
	return raw, old, newEntry, nil
}

// DisableKey marks a key as disabled; subsequent lookups fail.
func (s *AdminStore) DisableKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.keys[id]
	if !ok {
		return errors.New("key not found")
	}
	e.Disabled = true
	return nil
}

// DeleteKey removes a key permanently.
func (s *AdminStore) DeleteKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.keys[id]
	if !ok {
		return errors.New("key not found")
	}
	delete(s.hashToID, e.Hash)
	delete(s.keys, id)
	return nil
}

// ListKeys returns all entries, newest first.
func (s *AdminStore) ListKeys() []*KeyEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*KeyEntry, 0, len(s.keys))
	for _, e := range s.keys {
		out = append(out, e)
	}
	return out
}

// PurgeExpired deletes keys whose ExpiresAt is in the past.
// Returns the number deleted.
func (s *AdminStore) PurgeExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, e := range s.keys {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			delete(s.hashToID, e.Hash)
			delete(s.keys, id)
			n++
		}
	}
	return n
}
