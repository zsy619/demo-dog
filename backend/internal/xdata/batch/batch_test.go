package batch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memSink struct {
	mu      sync.Mutex
	entries [][]Entry
	err     error
}

func (m *memSink) Apply(ctx context.Context, e []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	cp := make([]Entry, len(e))
	copy(cp, e)
	m.entries = append(m.entries, cp)
	return nil
}

func TestPutFlush(t *testing.T) {
	s := &memSink{}
	w := New(s, 4, time.Hour)
	w.Put("a", []byte("1"))
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	n := len(s.entries)
	c := 0
	if n > 0 { c = len(s.entries[0]) }
	s.mu.Unlock()
	if n != 1 || c != 1 {
		t.Fatal("应 1 批 1 项")
	}
}

func TestAutoFlush(t *testing.T) {
	s := &memSink{}
	w := New(s, 2, time.Hour)
	w.Put("a", nil)
	w.Put("b", nil) // 达 cap
	time.Sleep(20 * time.Millisecond)
	s.mu.Lock()
	n := len(s.entries)
	s.mu.Unlock()
	if n == 0 {
		t.Fatal("应自动 flush")
	}
}

func TestDelete(t *testing.T) {
	s := &memSink{}
	w := New(s, 4, time.Hour)
	w.Delete("a")
	w.Flush()
	s.mu.Lock()
	op := s.entries[0][0].Op
	s.mu.Unlock()
	if op != OpDelete {
		t.Fatal("del")
	}
}

func TestPeriodicFlush(t *testing.T) {
	s := &memSink{}
	w := New(s, 100, 30*time.Millisecond)
	w.Put("a", nil)
	time.Sleep(60 * time.Millisecond)
	s.mu.Lock()
	n := len(s.entries)
	s.mu.Unlock()
	if n == 0 {
		t.Fatal("应定时 flush")
	}
	w.Close()
}

func TestLastError(t *testing.T) {
	s := &memSink{err: errors.New("x")}
	w := New(s, 1, time.Hour)
	w.Put("k", nil)
	w.Flush()
	if w.LastError() == nil {
		t.Fatal("应报错")
	}
}

func TestClose(t *testing.T) {
	s := &memSink{}
	w := New(s, 4, time.Hour)
	w.Put("a", nil)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	n := len(s.entries)
	s.mu.Unlock()
	if n == 0 {
		t.Fatal("close 应 flush")
	}
}
