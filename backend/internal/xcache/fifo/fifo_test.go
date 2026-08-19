package fifo

import (
	"testing"
)

func TestPutGet(t *testing.T) {
	c := New(2)
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
		t.Fatal("a 应被淘汰")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("b 应存活")
	}
}

func TestDelete(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	if !c.Delete("a") {
		t.Fatal("del")
	}
	if c.Delete("x") {
		t.Fatal("missing")
	}
}

func TestLen(t *testing.T) {
	c := New(2)
	if c.Len() != 0 {
		t.Fatal("空")
	}
	c.Put("a", 1)
	if c.Len() != 1 {
		t.Fatal("1")
	}
}

func TestCapacity(t *testing.T) {
	if New(2).Capacity() != 2 {
		t.Fatal("cap")
	}
}

func TestClear(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestKeys(t *testing.T) {
	c := New(3)
	c.Put("a", 1)
	c.Put("b", 2)
	k := c.Keys()
	if len(k) != 2 || k[0] != "a" || k[1] != "b" {
		t.Fatal("顺序错")
	}
}

func TestUpdate(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("a", 2)
	v, _ := c.Get("a")
	if v.(int) != 2 {
		t.Fatal("update")
	}
}
