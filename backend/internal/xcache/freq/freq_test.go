package freq

import "testing"

func TestIncCount(t *testing.T) {
	c := New(10)
	c.Inc("a")
	c.Inc("a")
	if c.Count("a") != 2 {
		t.Fatal("count", c.Count("a"))
	}
}

func TestHot(t *testing.T) {
	c := New(10)
	c.Inc("a")
	c.Inc("a")
	c.Inc("b")
	h := c.Hot(0)
	if len(h) != 2 || h[0].Key != "a" {
		t.Fatal("hot", h)
	}
}

func TestHot_Limit(t *testing.T) {
	c := New(10)
	c.Inc("a")
	c.Inc("b")
	h := c.Hot(1)
	if len(h) != 1 {
		t.Fatal("limit", h)
	}
}

func TestClear(t *testing.T) {
	c := New(10)
	c.Inc("a")
	c.Clear()
	if c.Count("a") != 0 {
		t.Fatal("clear")
	}
}

func TestRebuild(t *testing.T) {
	c := New(2)
	for i := 0; i < 100; i++ {
		c.Inc(string(rune('a' + i%5)))
	}
	if len(c.topHot) > 2 {
		t.Fatal("top", c.topHot)
	}
}
