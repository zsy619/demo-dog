// Package tscache 提供一个简单的键-时间序列缓存（用于监控）。
package tscache

import (
	"container/list"
	"sync"
	"time"
)

// Cache 按 key 保存最近 n 条带时间戳的样本。
type Cache struct {
	mu       sync.Mutex
	cap      int
	retain   time.Duration
	samples  map[string]*list.List
}

type sample struct {
	at time.Time
	v float64
}

// New 创建一个时间序列缓存。
func New(cap int, retain time.Duration) *Cache {
	if cap < 1 {
		cap = 256
	}
	if retain <= 0 {
		retain = time.Hour
	}
	return &Cache{cap: cap, retain: retain, samples: make(map[string]*list.List)}
}

// Add 添加一个样本。
func (c *Cache) Add(k string, v float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.samples[k]
	if !ok {
		l = list.New()
		c.samples[k] = l
	}
	l.PushBack(sample{at: time.Now(), v: v})
	for l.Len() > c.cap {
		front := l.Front()
		if front == nil {
			break
		}
		l.Remove(front)
	}
}

// Snapshot 返回某 key 的时间序列浅拷贝。
func (c *Cache) Snapshot(k string) []Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.samples[k]
	if !ok {
		return nil
	}
	cutoff := time.Now().Add(-c.retain)
	out := make([]Snapshot, 0, l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		s := e.Value.(sample)
		if s.at.After(cutoff) {
			out = append(out, Snapshot{At: s.at, Value: s.v})
		}
	}
	return out
}

// Snapshot 是单条样本。
type Snapshot struct {
	At    time.Time `json:"at"`
	Value float64   `json:"v"`
}

// Latest 返回最新一个样本。
func (c *Cache) Latest(k string) (Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.samples[k]
	if !ok || l.Len() == 0 {
		return Snapshot{}, false
	}
	back := l.Back()
	if back == nil {
		return Snapshot{}, false
	}
	s := back.Value.(sample)
	return Snapshot{At: s.at, Value: s.v}, true
}

// Keys 返回所有 key。
func (c *Cache) Keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.samples))
	for k := range c.samples {
		out = append(out, k)
	}
	return out
}

// Clear 清空某个 key。
func (c *Cache) Clear(k string) {
	c.mu.Lock()
	delete(c.samples, k)
	c.mu.Unlock()
}
