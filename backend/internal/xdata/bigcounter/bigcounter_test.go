package bigcounter

import "testing"

func TestAdd(t *testing.T) {
	c := New(0)
	c.Add(10)
	c.Add(5)
	v := c.Value()
	if v.Int64() != 15 {
		t.Fatal("v", v)
	}
}

func TestString(t *testing.T) {
	c := New(0)
	c.Add(42)
	if c.String() != "42" {
		t.Fatal("str", c.String())
	}
}

func TestStart(t *testing.T) {
	c := New(100)
	v := c.Value()
	if v.Int64() != 100 {
		t.Fatal("start")
	}
}
