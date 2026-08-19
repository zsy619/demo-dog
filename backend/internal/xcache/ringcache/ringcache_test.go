package ringcache

import "testing"

func TestPutGet(t *testing.T) {
	c := New[string, int](4)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatal("get")
	}
}

func TestOverflow(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 应被驱逐")
	}
}

func TestUpdate(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("a", 2)
	v, _ := c.Get("a")
	if v != 2 {
		t.Fatal("upd", v)
	}
}

func TestLen(t *testing.T) {
	c := New[string, int](4)
	c.Put("a", 1)
	c.Put("b", 2)
	if c.Len() != 2 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}
