package sizelim

import "testing"

func TestPutGet(t *testing.T) {
	c := New(1024)
	c.Put("a", "v", 4)
	v, ok := c.Get("a")
	if !ok || v.(string) != "v" {
		t.Fatal("get")
	}
}

func TestEviction(t *testing.T) {
	c := New(10)
	c.Put("a", 1, 4)
	c.Put("b", 2, 4)
	c.Put("c", 3, 4)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 应淘汰")
	}
}

func TestUpdate(t *testing.T) {
	c := New(1024)
	c.Put("a", 1, 4)
	c.Put("a", 2, 8)
	if v, _ := c.Get("a"); v.(int) != 2 {
		t.Fatal("update")
	}
	if c.Size() != 8 {
		t.Fatal("size")
	}
}

func TestSize(t *testing.T) {
	c := New(1024)
	c.Put("a", 1, 100)
	c.Put("b", 2, 200)
	if c.Size() != 300 {
		t.Fatal("size")
	}
}

func TestLen(t *testing.T) {
	c := New(1024)
	if c.Len() != 0 {
		t.Fatal("empty")
	}
	c.Put("a", 1, 1)
	if c.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	c := New(1024)
	c.Put("a", 1, 1)
	c.Clear()
	if c.Size() != 0 || c.Len() != 0 {
		t.Fatal("clear")
	}
}
