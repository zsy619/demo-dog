package tinycache

import "testing"

func TestSetGet(t *testing.T) {
	c := New(4)
	c.Set("a", "1")
	if v, _ := c.Get("a"); v != "1" {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	c := New(2)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3")
	if c.Len() > 2 {
		t.Fatal("len", c.Len())
	}
}

func TestDelete(t *testing.T) {
	c := New(2)
	c.Set("a", "1")
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestClear(t *testing.T) {
	c := New(4)
	c.Set("a", "1")
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}
