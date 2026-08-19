// Package nonce 提供一次性随机数（nonce）的生成与回放保护。
// 内部维护一个时间窗的滑动集合，避免 5 分钟内重复使用。
package nonce

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ErrReplay 在 nonce 已被使用或过长时返回。
var ErrReplay = errors.New("nonce: 重放或过期")

// Store 持有已用 nonce 的滑动窗口。
type Store struct {
	mu      sync.Mutex
	used    map[string]time.Time
	window  time.Duration
	nowFn   func() time.Time
}

// New 创建一个窗口为 window 的 Store。
func New(window time.Duration) *Store {
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &Store{used: make(map[string]time.Time), window: window, nowFn: time.Now}
}

// SetNow 用于测试时注入时间源。
func (s *Store) SetNow(fn func() time.Time) { s.nowFn = fn }

// Generate 返回一个新随机 nonce。
func Generate() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Check 校验 nonce：未用过且当前时间减去时间戳不超过窗口。
func (s *Store) Check(n string, ts int64) error {
	now := s.nowFn()
	if now.Sub(time.Unix(ts, 0)) > s.window {
		return ErrReplay
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(now)
	if _, ok := s.used[n]; ok {
		return ErrReplay
	}
	s.used[n] = now
	return nil
}

// Mark 主动加入一个 nonce（用于已生成的 nonce 预占）。
func (s *Store) Mark(n string) {
	s.mu.Lock()
	s.used[n] = s.nowFn()
	s.mu.Unlock()
}

// Len 返回当前窗口中的 nonce 数。
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(s.nowFn())
	return len(s.used)
}

func (s *Store) sweepLocked(now time.Time) {
	cutoff := now.Add(-s.window)
	for k, t := range s.used {
		if t.Before(cutoff) {
			delete(s.used, k)
		}
	}
}
