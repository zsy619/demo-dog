package lsm

// run.go:SortedRun 不可变有序键值层。
//
// SortedRun 由 StringTable.Flush 创建,只读不写。

import "sort"

// SortedRun 是磁盘上不可变的有序键/值 run。
type SortedRun struct {
	entries []Entry // 按 key 排序(只读)
}

// NewSortedRun 由 entries 创建 run(假设已按 key 排序)。
func NewSortedRun(entries []Entry) *SortedRun {
	cp := make([]Entry, len(entries))
	for i, e := range entries {
		cp[i] = Entry{Key: e.Key, Value: copyBytes(e.Value), Tombstone: e.Tombstone}
	}
	return &SortedRun{entries: cp}
}

// Get 返回与 Memtable.Get 相同的三元组。
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
