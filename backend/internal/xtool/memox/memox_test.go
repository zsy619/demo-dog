package memox

import "testing"

func TestSetGet(t *testing.T) {
	m := New()
	m.Set("a", 1, 100)
	v, ok := m.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestLastAccess(t *testing.T) {
	m := New()
	m.Set("a", 1, 100)
	ts, ok := m.LastAccess("a")
	if !ok || ts != 100 {
		t.Fatal("ts")
	}
}

func TestDelete(t *testing.T) {
	m := New()
	m.Set("a", 1, 100)
	m.Delete("a")
	if _, ok := m.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestClear(t *testing.T) {
	m := New()
	m.Set("a", 1, 100)
	m.Clear()
	if m.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestMiss(t *testing.T) {
	m := New()
	if _, ok := m.LastAccess("x"); ok {
		t.Fatal("miss")
	}
}
