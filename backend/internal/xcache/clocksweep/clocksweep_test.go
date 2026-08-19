package clocksweep

import "testing"

func TestPutGet(t *testing.T) {
	c := New(8)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestEvict(t *testing.T) {
	c := New(4)
	for i := 0; i < 10; i++ {
		c.Put(string(rune('a'+i)), i)
	}
	if c.Len() != 4 {
		t.Fatal("len", c.Len())
	}
}

func TestUpdate(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	c.Put("a", 2)
	v, _ := c.Get("a")
	if v.(int) != 2 {
		t.Fatal("upd")
	}
}

func TestMiss(t *testing.T) {
	c := New(4)
	if _, ok := c.Get("x"); ok {
		t.Fatal("miss")
	}
}

func TestProtect(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a") // 应被保护
	c.Put("c", 3)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a 应被保护")
	}
}
