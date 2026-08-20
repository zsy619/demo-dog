// Package lsm LSM 结构：内存 + 多层磁盘的写入优化存储。
package lsm

import (
	"sort"
	"sync"
)

// Entry 是一个键值对。已删除条目表示为
// with Tombstone=true.
type Entry struct {
	Key       string
	Value     []byte
	Tombstone bool
}

// Memtable 是由以下支持的内存有序键/值存储
// RedBlack-style skiplist. For Round 87 we use a sorted slice
// + binary search for simplicity.
type Memtable struct {
	mu      sync.RWMutex
	entries []Entry
	size    int
}

// NewMemtable 构造一个空 Memtable。
func NewMemtable() *Memtable {
	return &Memtable{}
}

// Put 插入或替换一个 key。
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

// Delete 将 key 标记为墓碑。
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

// Get 对活跃 key 返回 (value, true, false)，对缺失返回 (_, false,
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

// Len 返回条目数。
func (m *Memtable) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

// SortedRun 是磁盘上不可变的有序键/值 run。
type SortedRun struct {
	entries []Entry
}

// NewSortedRun 由 entries 创建 run（假设已
// sorted by key).
func NewSortedRun(entries []Entry) *SortedRun {
	cp := make([]Entry, len(entries))
	for i, e := range entries {
		cp[i] = Entry{Key: e.Key, Value: copyBytes(e.Value), Tombstone: e.Tombstone}
	}
	return &SortedRun{entries: cp}
}

// Get 返回与 Memtable.Get 相同的结果。
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

// StringTable 是 LSM 结构：memtable + 有序 run。
// Reads go newest-to-oldest.
type StringTable struct {
	mu      sync.RWMutex
	mem     *Memtable
	runs    []*SortedRun
}

// NewStringTable 构造一个空 LSM。
func NewStringTable() *StringTable {
	return &StringTable{mem: NewMemtable()}
}

// Put 将一个 key 插入 memtable。
func (s *StringTable) Put(key string, value []byte) {
	s.mu.Lock()
	s.mem.Put(key, value)
	s.mu.Unlock()
}

// Delete 在 memtable 中将 key 标记为已删除。
func (s *StringTable) Delete(key string) {
	s.mu.Lock()
	s.mem.Delete(key)
	s.mu.Unlock()
}

// Get 先从 memtable 读取，若未命中则按从旧到新
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

// Flush 将 memtable 移入新的有序 run 并清空
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

// MemLen 返回 memtable 大小。
func (s *StringTable) MemLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mem.Len()
}

// RunCount 返回有序 run 的数量。
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
