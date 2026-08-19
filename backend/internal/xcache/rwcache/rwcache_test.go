package rwcache

import "testing"

func TestPutGet(t *testing.T) {
	c := New()
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestStats(t *testing.T) {
	c := New()
	c.Put("a", 1)
	c.Get("a")
	c.Get("b")
	if c.Hits() != 1 || c.Misses() != 1 {
		t.Fatal("stats", c.Hits(), c.Misses())
	}
	r := c.HitRate()
	if r < 0.4 || r > 0.6 {
		t.Fatal("rate", r)
	}
}

func TestEmptyRate(t *testing.T) {
	c := New()
	if c.HitRate() != 0 {
		t.Fatal("empty rate")
	}
}

func TestDelete(t *testing.T) {
	c := New()
	c.Put("a", 1)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("del")
	}
}
