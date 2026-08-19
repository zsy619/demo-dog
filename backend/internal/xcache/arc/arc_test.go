package arc

import "testing"

func TestBasic(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	if v, ok := c.Get("a"); !ok || v.(int) != 1 {
		t.Fatal("get a")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("get b")
	}
}

func TestReplace(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if c.Len() > 2 {
		t.Fatal("len", c.Len())
	}
}

func TestUpdate(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("a", 2)
	if v, _ := c.Get("a"); v.(int) != 2 {
		t.Fatal("update")
	}
}

func TestPromotion(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Get("a")
	c.Put("d", 4)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a promote")
	}
}

func TestCap(t *testing.T) {
	c := New(8)
	if c.Cap() != 8 {
		t.Fatal("cap")
	}
}

func TestLenEmpty(t *testing.T) {
	c := New(2)
	if c.Len() != 0 {
		t.Fatal("empty")
	}
}
