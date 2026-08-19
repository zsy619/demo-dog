package lru2

import "testing"

func TestGet(t *testing.T) {
	c := New[string, int](4)
	c.Put("a", 1)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatal("get")
	}
}

func TestEviction(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 应淘汰")
	}
}

func TestUpdate(t *testing.T) {
	c := New[string, int](4)
	c.Put("a", 1)
	c.Put("a", 2)
	if v, _ := c.Get("a"); v != 2 {
		t.Fatal("update")
	}
}

func TestLen(t *testing.T) {
	c := New[string, int](4)
	c.Put("a", 1)
	if c.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	c := New[string, int](4)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestKeys(t *testing.T) {
	c := New[string, int](4)
	c.Put("a", 1)
	c.Put("b", 2)
	k := c.Keys()
	if len(k) != 2 {
		t.Fatal("keys")
	}
	if k[0] != "b" || k[1] != "a" {
		t.Fatal("order")
	}
}

func TestCap(t *testing.T) {
	c := New[string, int](7)
	if c.Cap() != 7 {
		t.Fatal("cap")
	}
}
