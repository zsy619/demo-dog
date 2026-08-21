// Package secretrot 密钥轮换：定期更新密钥并保留历史。
package secretrot

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"
)

// Store 是一个按租户划分的密钥存储，并支持自动轮换。
type Store struct {
	mu       sync.RWMutex
	secrets  map[string]*entry
	interval time.Duration
	now      func() time.Time
}

type entry struct {
	value     []byte
	created   time.Time
	expires   time.Time
	rotations int
}

// New 创建一个带轮换间隔的 Store。
func New(interval time.Duration) *Store {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Store{
		secrets:  make(map[string]*entry),
		interval: interval,
		now:      time.Now,
	}
}

// WithTime 覆盖用于测试的时间源。
func (s *Store) WithTime(now func() time.Time) *Store {
	s.now = now
	return s
}

// ErrNotFound 在租户未知时返回。
var ErrNotFound = errors.New("secret not found")

// Get 返回 current secret for 租户. If the secret
// is expired, it is rotated first.
func (s *Store) Get(tenant string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.secrets[tenant]
	if !ok {
		return nil, ErrNotFound
	}
	if !s.now().Before(e.expires) {
		s.rotateLocked(tenant, e)
	}
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, nil
}

// Set 安装或替换一个租户的密钥。
func (s *Store) Set(tenant string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[tenant] = &entry{
		value:   append([]byte{}, value...),
		created: s.now(),
		expires: s.now().Add(s.interval),
	}
}

// Rotate 强制重新生成某个租户的密钥。
func (s *Store) Rotate(tenant string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.secrets[tenant]
	if !ok {
		return ErrNotFound
	}
	s.rotateLocked(tenant, e)
	return nil
}

// Delete 移除某个租户的密钥。
func (s *Store) Delete(tenant string) {
	s.mu.Lock()
	delete(s.secrets, tenant)
	s.mu.Unlock()
}

// Snapshot 返回所有租户的轮换元数据。
func (s *Store) Snapshot() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]time.Time, len(s.secrets))
	for k, e := range s.secrets {
		out[k] = e.expires
	}
	return out
}

// Rotations 返回指定租户的轮换次数。
func (s *Store) Rotations(tenant string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.secrets[tenant]
	if !ok {
		return 0
	}
	return e.rotations
}

func (s *Store) rotateLocked(tenant string, e *entry) {
	buf := make([]byte, 32)
	rand.Read(buf)
	e.value = buf
	e.created = s.now()
	e.expires = s.now().Add(s.interval)
	e.rotations++
}
