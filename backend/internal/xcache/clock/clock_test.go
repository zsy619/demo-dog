package clock

import "testing"

func TestPutGet(t *testing.T) {
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
	if c.Len() > 2 {
		t.Fatal("应不超容量")
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
