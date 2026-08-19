// Package versionstore 提供简单的多版本数据存储（最后 N 个版本可回溯）。
package versionstore

import (
	"container/list"
	"sync"
)

// Store 是一个多版本 KV 存储。
type Store struct {
	mu     sync.RWMutex
	max    int
	data   map[string]*list.List // 元素 value (ver, val)
}

// New 创建一个最多保留 max 个版本的 Store。
func New(max int) *Store {
	if max < 1 {
		max = 5
	}
	return &Store{max: max, data: make(map[string]*list.List)}
}

type entry struct {
	ver uint64
	val any
}

// Set 设置键值。
func (s *Store) Set(key string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.data[key]
	if !ok {
		l = list.New()
		s.data[key] = l
	}
	var next uint64 = 1
	if back := l.Back(); back != nil {
		next = back.Value.(*entry).ver + 1
	}
	l.PushBack(&entry{ver: next, val: v})
	for l.Len() > s.max {
		front := l.Front()
		if front == nil {
			break
		}
		l.Remove(front)
	}
}

// Get 读取最新值。
func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.data[key]
	if !ok || l.Len() == 0 {
		return nil, false
	}
	back := l.Back()
	if back == nil {
		return nil, false
	}
	return back.Value.(*entry).val, true
}

// GetAt 读取某版本的值。
func (s *Store) GetAt(key string, ver uint64) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.data[key]
	if !ok {
		return nil, false
	}
	for e := l.Back(); e != nil; e = e.Prev() {
		if e.Value.(*entry).ver == ver {
			return e.Value.(*entry).val, true
		}
	}
	return nil, false
}

// Versions 返回 key 的所有版本号（升序）。
func (s *Store) Versions(key string) []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.data[key]
	if !ok {
		return nil
	}
	out := make([]uint64, 0, l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(*entry).ver)
	}
	return out
}

// Delete 删除 key。
func (s *Store) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}
