package lrumap

import "testing"

func TestPutGet(t *testing.T) {
	m := New(8)
	m.Put("a", 1)
	v, ok := m.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	m := New(2)
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)
	if _, ok := m.Get("a"); ok {
		t.Fatal("evict")
	}
}

func TestDelete(t *testing.T) {
	m := New(4)
	m.Put("a", 1)
	m.Delete("a")
	if _, ok := m.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestLen(t *testing.T) {
	m := New(4)
	m.Put("a", 1)
	if m.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	m := New(4)
	m.Put("a", 1)
	m.Clear()
	if m.Len() != 0 {
		t.Fatal("clear")
	}
}
