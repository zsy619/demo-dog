// Package softdel 软删除：标记删除而非真正删除，支持恢复。
package softdel

import (
	"errors"
	"sync"
	"time"
)

// Record 是带软删除标记的存储条目。
type Record struct {
	ID        string
	Data      []byte
	DeletedAt time.Time
	Exists    bool
}

// Store 保留带可选 TTL + 软删除的记录。
type Store struct {
	mu       sync.RWMutex
	records  map[string]*Record
	ttl      time.Duration
	now      func() time.Time
}

// ErrNotFound 在 ID 缺失时返回。
var ErrNotFound = errors.New("record not found")

// ErrAlreadyDeleted 在删除墓碑记录时返回。
// record.
var ErrAlreadyDeleted = errors.New("already deleted")

// New 以 TTL 创建一个 Store。
func New(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{records: make(map[string]*Record), ttl: ttl, now: time.Now}
}

// WithTime 覆盖测试的时间源。
func (s *Store) WithTime(now func() time.Time) *Store {
	s.now = now
	return s
}

// Put 按 id 存储一条记录。
func (s *Store) Put(id string, data []byte) {
	s.mu.Lock()
	s.records[id] = &Record{ID: id, Data: data, Exists: true}
	s.mu.Unlock()
}

// Get 返回记录的副本。若不存在则返回 ErrNotFound。
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

// Delete 软删除一条记录。
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

// Restore 清除软删除标记。
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

// Reclaim 删除早于 TTL 的软删除记录。
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

// List 返回所有活跃记录。
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

// Len 返回总记录数（活跃 + 墓碑）。
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Live 返回活跃（未删除）记录数。
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
