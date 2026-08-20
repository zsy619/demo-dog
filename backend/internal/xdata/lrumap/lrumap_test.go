package lrumap

import "testing"

func TestPutGet(t *testing.T) {
	m := New[string, int](8)
	m.Put("a", 1)
	v, ok := m.Get("a")
	if !ok || v != 1 {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	m := New[string, int](2)
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)
	if _, ok := m.Get("a"); ok {
		t.Fatal("evict")
	}
}

func TestDelete(t *testing.T) {
	m := New[string, int](4)
	m.Put("a", 1)
	m.Delete("a")
	if _, ok := m.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestLen(t *testing.T) {
	m := New[string, int](4)
	m.Put("a", 1)
	if m.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	m := New[string, int](4)
	m.Put("a", 1)
	m.Clear()
	if m.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestKeys(t *testing.T) {
	m := New[string, int](4)
	m.Put("a", 1)
	m.Put("b", 2)
	ks := m.Keys()
	if len(ks) != 2 || ks[0] != "b" {
		t.Fatal("keys", ks)
	}
}

func TestBytesValue(t *testing.T) {
	// 替代之前 lrukv 的 string->[]byte 场景
	m := New[string, []byte](4)
	m.Put("a", []byte("hi"))
	v, ok := m.Get("a")
	if !ok || string(v) != "hi" {
		t.Fatal("bytes")
	}
}
