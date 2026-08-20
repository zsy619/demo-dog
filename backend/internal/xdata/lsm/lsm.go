// Package lsm LSM 结构：内存 + 多层磁盘的写入优化存储。
package lsm

import (
	"sort"
	"sync"
)

// Entry is one key/value pair. Deleted entries are represented
// with Tombstone=true.
type Entry struct {
	Key       string
	Value     []byte
	Tombstone bool
}

// Memtable is an in-memory sorted key/value store backed by a
// RedBlack-style skiplist. For Round 87 we use a sorted slice
// + binary search for simplicity.
type Memtable struct {
	mu      sync.RWMutex
	entries []Entry
	size    int
}

// NewMemtable constructs an empty Memtable.
func NewMemtable() *Memtable {
	return &Memtable{}
}

// Put inserts or replaces a key.
func (m *Memtable) Put(key string, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := sort.Search(len(m.entries), func(i int) bool {
		return m.entries[i].Key >= key
	})
	if idx < len(m.entries) && m.entries[idx].Key == key {
		m.entries[idx].Value = copyBytes(value)
		m.entries[idx].Tombstone = false
		return
	}
	m.entries = append(m.entries, Entry{})
	copy(m.entries[idx+1:], m.entries[idx:])
	m.entries[idx] = Entry{Key: key, Value: copyBytes(value)}
	m.size++
}

// Delete marks the key as a tombstone.
func (m *Memtable) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := sort.Search(len(m.entries), func(i int) bool {
		return m.entries[i].Key >= key
	})
	if idx < len(m.entries) && m.entries[idx].Key == key {
		m.entries[idx].Tombstone = true
		m.entries[idx].Value = nil
		return
	}
	m.entries = append(m.entries, Entry{})
	copy(m.entries[idx+1:], m.entries[idx:])
	m.entries[idx] = Entry{Key: key, Tombstone: true}
	m.size++
}

// Get returns (value, true, false) for a live key, (_, false,
// false) for missing, and (_, false, true) for tombstone.
func (m *Memtable) Get(key string) ([]byte, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx := sort.Search(len(m.entries), func(i int) bool {
		return m.entries[i].Key >= key
	})
	if idx >= len(m.entries) || m.entries[idx].Key != key {
		return nil, false, false
	}
	if m.entries[idx].Tombstone {
		return nil, false, true
	}
	return copyBytes(m.entries[idx].Value), true, false
}

// Len returns the number of entries.
func (m *Memtable) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

// SortedRun is an immutable sorted key/value run on disk.
type SortedRun struct {
	entries []Entry
}

// NewSortedRun creates a run from entries (assumed already
// sorted by key).
func NewSortedRun(entries []Entry) *SortedRun {
	cp := make([]Entry, len(entries))
	for i, e := range entries {
		cp[i] = Entry{Key: e.Key, Value: copyBytes(e.Value), Tombstone: e.Tombstone}
	}
	return &SortedRun{entries: cp}
}

// Get returns the same shape as Memtable.Get.
func (r *SortedRun) Get(key string) ([]byte, bool, bool) {
	idx := sort.Search(len(r.entries), func(i int) bool {
		return r.entries[i].Key >= key
	})
	if idx >= len(r.entries) || r.entries[idx].Key != key {
		return nil, false, false
	}
	if r.entries[idx].Tombstone {
		return nil, false, true
	}
	return copyBytes(r.entries[idx].Value), true, false
}

// StringTable is the LSM structure: memtable + sorted runs.
// Reads go newest-to-oldest.
type StringTable struct {
	mu      sync.RWMutex
	mem     *Memtable
	runs    []*SortedRun
}

// NewStringTable constructs an empty LSM.
func NewStringTable() *StringTable {
	return &StringTable{mem: NewMemtable()}
}

// Put inserts a key into the memtable.
func (s *StringTable) Put(key string, value []byte) {
	s.mu.Lock()
	s.mem.Put(key, value)
	s.mu.Unlock()
}

// Delete marks a key deleted in the memtable.
func (s *StringTable) Delete(key string) {
	s.mu.Lock()
	s.mem.Delete(key)
	s.mu.Unlock()
}

// Get reads from memtable then runs oldest-to-newest if not
// found.
func (s *StringTable) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	val, ok, tomb := s.mem.Get(key)
	if tomb {
		s.mu.RUnlock()
		return nil, false
	}
	if ok {
		s.mu.RUnlock()
		return val, true
	}
	for i := 0; i < len(s.runs); i++ {
		v, ok, tomb := s.runs[i].Get(key)
		if tomb {
			s.mu.RUnlock()
			return nil, false
		}
		if ok {
			s.mu.RUnlock()
			return v, true
		}
	}
	s.mu.RUnlock()
	return nil, false
}

// Flush moves the memtable into a new sorted run and clears
// the memtable. Returns the run seq.
func (s *StringTable) Flush() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := make([]Entry, s.mem.size)
	copy(entries, s.mem.entries)
	run := NewSortedRun(entries)
	s.runs = append([]*SortedRun{run}, s.runs...)
	s.mem = NewMemtable()
	return len(s.runs)
}

// MemLen returns the memtable size.
func (s *StringTable) MemLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mem.Len()
}

// RunCount returns the number of sorted runs.
func (s *StringTable) RunCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runs)
}

func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
