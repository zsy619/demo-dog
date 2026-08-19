// Package feature_flag 提供一个内存版特性开关存储。
package feature_flag

import "sync"

// Flag 是一个特性开关的状态。
type Flag struct {
	On        bool   `json:"on"`
	Rollout   int    `json:"rollout"`   // 0-100 灰度比例
	Desc      string `json:"desc"`
}

// Store 是一个线程安全的特性开关集合。
type Store struct {
	mu    sync.RWMutex
	flags map[string]Flag
}

// New 创建一个空 Store。
func New() *Store { return &Store{flags: make(map[string]Flag)} }

// Set 写入 flag。
func (s *Store) Set(name string, f Flag) {
	s.mu.Lock()
	s.flags[name] = f
	s.mu.Unlock()
}

// Get 读取 flag。
func (s *Store) Get(name string) (Flag, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.flags[name]
	return f, ok
}

// IsEnabled 检查 flag 是否启用。
func (s *Store) IsEnabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.flags[name]
	if !ok {
		return false
	}
	return f.On
}

// Enable 启用 flag。
func (s *Store) Enable(name string) {
	s.mu.Lock()
	f := s.flags[name]
	f.On = true
	s.flags[name] = f
	s.mu.Unlock()
}

// Disable 关闭 flag。
func (s *Store) Disable(name string) {
	s.mu.Lock()
	f := s.flags[name]
	f.On = false
	s.flags[name] = f
	s.mu.Unlock()
}

// Delete 删除 flag。
func (s *Store) Delete(name string) {
	s.mu.Lock()
	delete(s.flags, name)
	s.mu.Unlock()
}

// All 列出所有 flag。
func (s *Store) All() map[string]Flag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Flag, len(s.flags))
	for k, v := range s.flags {
		out[k] = v
	}
	return out
}
