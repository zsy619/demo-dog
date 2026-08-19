// Package recency 跟踪每个 key 的最近访问时间戳。
package recency

import "sync"

// Tracker 是一个线程安全的 recency 跟踪器。
type Tracker struct {
	mu sync.RWMutex
	m  map[string]int64
}

// New 创建空 Tracker。
func New() *Tracker { return &Tracker{m: make(map[string]int64)} }

// Touch 把 key 的时间戳设为 ts。
func (t *Tracker) Touch(k string, ts int64) {
	t.mu.Lock()
	t.m[k] = ts
	t.mu.Unlock()
}

// Get 读取 key 的时间戳。
func (t *Tracker) Get(k string) (int64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.m[k]
	return v, ok
}

// Len 返回键数。
func (t *Tracker) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.m)
}

// Oldest 返回最久未访问的 (k, ts)。
func (t *Tracker) Oldest() (string, int64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var k string
	var ts int64 = 1<<63 - 1
	found := false
	for kk, tt := range t.m {
		if tt < ts {
			ts = tt
			k = kk
			found = true
		}
	}
	if !found {
		return "", 0, false
	}
	return k, ts, true
}

// PurgeOlderThan 删除 ts 之前的全部键，返回删除数。
func (t *Tracker) PurgeOlderThan(ts int64) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for k, v := range t.m {
		if v < ts {
			delete(t.m, k)
			n++
		}
	}
	return n
}

// Delete 删除一个 key。
func (t *Tracker) Delete(k string) {
	t.mu.Lock()
	delete(t.m, k)
	t.mu.Unlock()
}
