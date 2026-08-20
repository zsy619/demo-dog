// Package softdel 软删除：标记删除而非真正删除，支持恢复。
package softdel

import (
	"errors"
	"sync"
	"time"
)

// Record is a stored entry with a soft-delete marker.
type Record struct {
	ID        string
	Data      []byte
	DeletedAt time.Time
	Exists    bool
}

// Store keeps records with optional TTL + soft-delete.
type Store struct {
	mu       sync.RWMutex
	records  map[string]*Record
	ttl      time.Duration
	now      func() time.Time
}

// ErrNotFound is returned when the ID is missing.
var ErrNotFound = errors.New("record not found")

// ErrAlreadyDeleted is returned when deleting a tombstoned
// record.
var ErrAlreadyDeleted = errors.New("already deleted")

// New creates a Store with TTL.
func New(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{records: make(map[string]*Record), ttl: ttl, now: time.Now}
}

// WithTime overrides the time source for tests.
func (s *Store) WithTime(now func() time.Time) *Store {
	s.now = now
	return s
}

// Put stores a record by id.
func (s *Store) Put(id string, data []byte) {
	s.mu.Lock()
	s.records[id] = &Record{ID: id, Data: data, Exists: true}
	s.mu.Unlock()
}

// Get returns a copy of the record. Returns ErrNotFound if
// soft-deleted or missing.
func (s *Store) Get(id string) (*Record, error) {
	s.mu.RLock()
	rec, ok := s.records[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	if !rec.Exists {
		return nil, ErrNotFound
	}
	cp := *rec
	cp.Data = append([]byte{}, rec.Data...)
	return &cp, nil
}

// Delete soft-deletes a record.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return ErrNotFound
	}
	if !rec.Exists {
		return ErrAlreadyDeleted
	}
	rec.Exists = false
	rec.DeletedAt = s.now()
	return nil
}

// Restore clears the soft-delete marker.
func (s *Store) Restore(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return ErrNotFound
	}
	if rec.Exists {
		return nil
	}
	rec.Exists = true
	rec.DeletedAt = time.Time{}
	return nil
}

// Reclaim drops soft-deleted records older than the TTL.
// Returns the number reclaimed.
func (s *Store) Reclaim() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.now().Add(-s.ttl)
	n := 0
	for id, rec := range s.records {
		if !rec.Exists && rec.DeletedAt.Before(cutoff) {
			delete(s.records, id)
			n++
		}
	}
	return n
}

// List returns all live records.
func (s *Store) List() []*Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Record, 0, len(s.records))
	for _, rec := range s.records {
		if rec.Exists {
			cp := *rec
			cp.Data = append([]byte{}, rec.Data...)
			out = append(out, &cp)
		}
	}
	return out
}

// Len returns the total record count (live + tombstoned).
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Live returns the live (non-deleted) record count.
func (s *Store) Live() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, rec := range s.records {
		if rec.Exists {
			n++
		}
	}
	return n
}
