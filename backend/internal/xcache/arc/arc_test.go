package arc

import "testing"

func TestPutGet(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestGetMissing(t *testing.T) {
	c := New(4)
	if _, ok := c.Get("x"); ok {
		t.Fatal("missing")
	}
}

func TestUpdate(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	c.Put("a", 2)
	v, _ := c.Get("a")
	if v.(int) != 2 {
		t.Fatal("update")
	}
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
	c.Put("b", 2)
	if c.Len() != 2 {
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
