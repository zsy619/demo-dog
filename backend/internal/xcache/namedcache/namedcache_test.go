package namedcache

import "testing"

func TestPutGet(t *testing.T) {
	c := NewCache(4)
	c.Put("a", 1)
	v, _ := c.Get("a")
	if v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	c := NewCache(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("evict")
	}
}

func TestManager(t *testing.T) {
	m := NewManager(8)
	c := m.Get("users")
	c.Put("u1", 1)
	c2 := m.Get("users")
	v, _ := c2.Get("u1")
	if v.(int) != 1 {
		t.Fatal("shared")
	}
}

func TestNames(t *testing.T) {
	m := NewManager(8)
	m.Get("a")
	m.Get("b")
	if len(m.Names()) != 2 {
		t.Fatal("names")
	}
}
