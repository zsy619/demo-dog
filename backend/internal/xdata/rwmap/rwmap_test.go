package rwmap

import "testing"

func TestPutGet(t *testing.T) {
	m := New()
	m.Put("a", 1)
	v, ok := m.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestKeys(t *testing.T) {
	m := New()
	m.Put("a", 1)
	m.Put("b", 2)
	if len(m.Keys()) != 2 {
		t.Fatal("keys")
	}
}

func TestDelete(t *testing.T) {
	m := New()
	m.Put("a", 1)
	m.Delete("a")
	if _, ok := m.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestClear(t *testing.T) {
	m := New()
	m.Put("a", 1)
	m.Clear()
	if m.Len() != 0 {
		t.Fatal("clear")
	}
}
