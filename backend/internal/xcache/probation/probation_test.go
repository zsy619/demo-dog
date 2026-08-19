package probation

import "testing"

func TestPutGet(t *testing.T) {
	c := New(8)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestPromote(t *testing.T) {
	c := New(8)
	c.Put("a", 1)
	c.Get("a") // 触发晋升到 protected
	if _, ok := c.main["a"]; ok {
		t.Fatal("应已晋升")
	}
	if _, ok := c.prot["a"]; !ok {
		t.Fatal("应已在 protected")
	}
}

func TestEvict(t *testing.T) {
	c := New(4)
	for i := 0; i < 20; i++ {
		c.Put(string(rune('a'+i)), i)
	}
	if c.Len() > 5 {
		t.Fatal("len", c.Len())
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

func TestClear(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}
