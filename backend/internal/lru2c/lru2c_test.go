package lru2c

import (
	"testing"
)

func TestPutGet(t *testing.T) {
	c := New[string, int](3)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatal("get")
	}
}

func TestGet_Miss(t *testing.T) {
	c := New[string, int](3)
	if _, ok := c.Get("missing"); ok {
		t.Fatal("miss")
	}
}

func TestEviction(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("should evict a")
	}
}

func TestSecondChance(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")
	c.Put("c", 3)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should survive")
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should evict")
	}
}

func TestSecondChance_AllReferenced(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")
	c.Get("b")
	c.Put("c", 3)
	if c.Len() != 2 {
		t.Fatal("cap")
	}
}

func TestUpdate(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("a", 2)
	v, _ := c.Get("a")
	if v != 2 {
		t.Fatal("update")
	}
}

func TestPeek(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	v, ok := c.Peek("a")
	if !ok || v != 1 {
		t.Fatal("peek")
	}
}

func TestDelete(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("delete")
	}
}

func TestStats(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Get("a")
	c.Get("missing")
	s := c.Stats()
	if s.Hits != 1 || s.Misses != 1 {
		t.Fatal("stats")
	}
}

func TestLen(t *testing.T) {
	c := New[string, int](3)
	if c.Len() != 0 {
		t.Fatal("empty")
	}
	c.Put("a", 1)
	if c.Len() != 1 {
		t.Fatal("after put")
	}
}
