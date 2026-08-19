package seqcache

import "testing"

func TestPutGet(t *testing.T) {
	c := New(8)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("evict")
	}
}

func TestClear(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestMiss(t *testing.T) {
	c := New(4)
	if _, ok := c.Get("x"); ok {
		t.Fatal("miss")
	}
}
