// Package dualstore 提供主备模式的 KV 存储：写入双份，读取优先主。
package dualstore

import "sync"

// Store 维护 primary 与 secondary 两份数据。
type Store struct {
	mu      sync.RWMutex
	primary map[string]any
	backup  map[string]any
}

// New 创建一个主备存储。
func New() *Store {
	return &Store{
		primary: make(map[string]any),
		backup:  make(map[string]any),
	}
}

// Set 写入两边。
func (s *Store) Set(k string, v any) {
	s.mu.Lock()
	s.primary[k] = v
	s.backup[k] = v
	s.mu.Unlock()
}

// Get 优先从 primary 读，缺失则从 backup。
func (s *Store) Get(k string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.primary[k]; ok {
		return v, true
	}
	if v, ok := s.backup[k]; ok {
		return v, true
	}
	return nil, false
}

// Delete 双删。
func (s *Store) Delete(k string) {
	s.mu.Lock()
	delete(s.primary, k)
	delete(s.backup, k)
	s.mu.Unlock()
}

// Failover 把 backup 提升为 primary，清空原 primary。
func (s *Store) Failover() {
	s.mu.Lock()
	s.primary = s.backup
	s.backup = make(map[string]any)
	s.mu.Unlock()
}

// Sync 把 primary 同步到 backup。
func (s *Store) Sync() {
	s.mu.Lock()
	s.backup = make(map[string]any, len(s.primary))
	for k, v := range s.primary {
		s.backup[k] = v
	}
	s.mu.Unlock()
}

// Len 返回元素数（primary）。
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.primary)
}

// DiffCount 返回 primary 与 backup 不一致的键数。
func (s *Store) DiffCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for k, v := range s.primary {
		if bv, ok := s.backup[k]; !ok || bv != v {
			n++
		}
	}
	for k := range s.backup {
		if _, ok := s.primary[k]; !ok {
			n++
		}
	}
	return n
}
