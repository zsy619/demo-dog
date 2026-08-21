package lsm

// memtable.go:Memtable 内存有序键值存储。
//
// 用排序切片 + 二分搜索实现;Round 87 简化版,未来可换 skiplist。

import (
	"sort"
	"sync"
)

// Memtable 是内存有序键/值存储。
//
// 当前实现使用排序切片 + 二分搜索;后续可替换为 skiplist 以获得更好的写入性能。
type Memtable struct {
	mu      sync.RWMutex // 保护 entries
	entries []Entry      // 按 key 排序
	size    int           // 条目数(等价于 len(entries))
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

// Get 返回 (value, found, tombstone)。
//
//   - (val, true, false)  命中活跃条目
//   - (nil, false, false) 未命中
//   - (nil, false, true)  命中墓碑条目(视为已删除)
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
