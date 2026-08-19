package syncmap

import "testing"

func TestSetGet(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	v, _ := m.Get("a")
	if v != 1 {
		t.Fatal("get", v)
	}
}

func TestGetMiss(t *testing.T) {
	m := New[string, int]()
	if _, ok := m.Get("x"); ok {
		t.Fatal("miss")
	}
}

func TestHas(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	if !m.Has("a") {
		t.Fatal("has")
	}
}

func TestDelete(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Delete("a")
	if m.Has("a") {
		t.Fatal("del")
	}
}

func TestKeys(t *testing.T) {
	m := New[int, string]()
	m.Set(1, "a")
	m.Set(2, "b")
	if len(m.Keys()) != 2 {
		t.Fatal("keys")
	}
}

func TestClear(t *testing.T) {
	m := New[int, string]()
	m.Set(1, "a")
	m.Clear()
	if m.Len() != 0 {
		t.Fatal("clear")
	}
}
