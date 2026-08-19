package lfu

import "testing"

func TestGet(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestEviction(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 应淘汰")
	}
}

func TestFreq(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")
	c.Get("a")
	c.Get("b")
	c.Put("c", 3)
	c.Put("d", 4)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a 高频 应保留")
	}
}

func TestLen(t *testing.T) {
	c := New(4)
	if c.Len() != 0 {
		t.Fatal("empty")
	}
	c.Put("a", 1)
	if c.Len() != 1 {
		t.Fatal("len")
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

func TestCap(t *testing.T) {
	c := New(7)
	if c.Cap() != 7 {
		t.Fatal("cap")
	}
}
