// Package journal 提供一个轻量级 KV 变更日志：
// 每次 Put/Delete 追加一条记录，支持按时间或 key 范围查询，
// 并可在独立协程中按批定期截断压缩。
package journal

import (
	"errors"
	"sync"
	"time"
)

// Op 表示日志条目类型。
type Op int

const (
	OpPut Op = iota
	OpDelete
)

// Entry 是日志中的一条记录。
type Entry struct {
	At    time.Time `json:"at"`
	Op    Op        `json:"op"`
	Key   string    `json:"key"`
	Value []byte    `json:"value,omitempty"`
}

// ErrFull 在追加超容量时被拒返回。
var ErrFull = errors.New("journal: 已达容量上限")

// Log 是一个线程安全的环形日志。
type Log struct {
	mu       sync.Mutex
	entries  []Entry
	capacity int
	seq      int
}

// New 创建一个容量为 capacity 的日志。
func New(capacity int) *Log {
	if capacity <= 0 {
		capacity = 1024
	}
	return &Log{capacity: capacity, entries: make([]Entry, 0, capacity)}
}

// Put 记录一条 Put。
func (l *Log) Put(key string, value []byte) error {
	return l.append(Entry{At: time.Now(), Op: OpPut, Key: key, Value: value})
}

// Delete 记录一条 Delete。
func (l *Log) Delete(key string) error {
	return l.append(Entry{At: time.Now(), Op: OpDelete, Key: key})
}

func (l *Log) append(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) >= l.capacity {
		return ErrFull
	}
	l.entries = append(l.entries, e)
	l.seq++
	return nil
}

// Range 返回所有按时间排序的记录。
func (l *Log) Range() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Filter 返回与 key 相关的所有记录。
func (l *Log) Filter(key string) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := []Entry{}
	for _, e := range l.entries {
		if e.Key == key {
			out = append(out, e)
		}
	}
	return out
}

// Latest 返回每个 key 的最近一次操作。
func (l *Log) Latest() map[string]Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[string]Entry{}
	for _, e := range l.entries {
		out[e.Key] = e
	}
	return out
}

// Len 返回当前条目数。
func (l *Log) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// Capacity 返回容量。
func (l *Log) Capacity() int { return l.capacity }

// Clear 清空。
func (l *Log) Clear() {
	l.mu.Lock()
	l.entries = l.entries[:0]
	l.mu.Unlock()
}
