package dualstore

import "testing"

func TestSetGet(t *testing.T) {
	s := New()
	s.Set("a", 1)
	v, ok := s.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestDelete(t *testing.T) {
	s := New()
	s.Set("a", 1)
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestFailover(t *testing.T) {
	s := New()
	s.Set("a", 1)
	s.Set("b", 2)
	s.mu.Lock()
	delete(s.primary, "b")
	s.mu.Unlock()
	if _, ok := s.Get("b"); !ok {
		t.Fatal("backup 仍应有 b")
	}
	s.Failover()
	if s.Len() != 2 {
		t.Fatal("长度", s.Len())
	}
}

func TestSync(t *testing.T) {
	s := New()
	s.Set("a", 1)
	s.mu.Lock()
	delete(s.backup, "a")
	s.mu.Unlock()
	if s.DiffCount() != 1 {
		t.Fatal("diff")
	}
	s.Sync()
	if s.DiffCount() != 0 {
		t.Fatal("sync")
	}
}

func TestMiss(t *testing.T) {
	s := New()
	if _, ok := s.Get("x"); ok {
		t.Fatal("miss")
	}
}
