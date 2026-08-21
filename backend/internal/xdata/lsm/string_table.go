package lsm

// string_table.go:StringTable 是 LSM 结构(memtable + 多层 run)。
//
// 读取路径:先 memtable,再按从新到旧遍历 runs;
// 写入路径:仅写入 memtable,满了由 Flush 转入新 run。

import "sync"

// StringTable 是 LSM 结构:memtable + 有序 run。
//
// 读取按从新到旧遍历;runs[0] 是最新一次 Flush 产生的层。
type StringTable struct {
	mu   sync.RWMutex // 保护 mem / runs
	mem  *Memtable    // 当前活跃 memtable
	runs []*SortedRun // 不可变 run,从新到旧
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

// Get 先从 memtable 读取,若未命中则按从新到旧遍历 runs。
//
// 一旦在某一层找到(无论活跃或墓碑),立即返回。
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

// Flush 将 memtable 移入新的有序 run 并清空 memtable。
//
// 返回新的 run 总数。
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
