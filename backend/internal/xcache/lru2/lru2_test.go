package lru2

import (
	"testing"
)

func TestPutGet(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestPromotion(t *testing.T) {
	c := New(10)
	c.Put("a", 1)
	c.Put("b", 2) // 让 a 不被立即淘汰
	c.Get("a") // 二次访问
	_ = c.Stats()
}

func TestEviction(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 应被淘汰")
	}
}

func TestLen(t *testing.T) {
	c := New(4)
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

func TestStats(t *testing.T) {
	c := New(10)
	c.Put("a", 1)
	s := c.Stats()
	if s.Capacity != 10 {
		t.Fatal("cap")
	}
}
