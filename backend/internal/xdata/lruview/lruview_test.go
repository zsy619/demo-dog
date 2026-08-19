package lruview

import "testing"

func TestPutGet(t *testing.T) {
	l := New[string, int](2, nil)
	l.Put("a", 1)
	v, ok := l.Get("a")
	if !ok || v != 1 {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	evicted := []string{}
	l := New[string, int](2, func(k string, _ int) { evicted = append(evicted, k) })
	l.Put("a", 1)
	l.Put("b", 2)
	l.Put("c", 3)
	if len(evicted) != 1 || evicted[0] != "a" {
		t.Fatal("evict")
	}
}

func TestLen(t *testing.T) {
	l := New[string, int](4, nil)
	l.Put("a", 1)
	if l.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	l := New[string, int](4, nil)
	l.Put("a", 1)
	l.Clear()
	if l.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestKeys(t *testing.T) {
	l := New[string, int](4, nil)
	l.Put("a", 1)
	l.Put("b", 2)
	l.Get("a") // a 提升
	keys := l.Keys()
	if len(keys) != 2 || keys[0] != "a" {
		t.Fatal("keys")
	}
}

func TestUpdate(t *testing.T) {
	l := New[string, int](4, nil)
	l.Put("a", 1)
	l.Put("a", 2)
	v, _ := l.Get("a")
	if v != 2 {
		t.Fatal("update")
	}
}
