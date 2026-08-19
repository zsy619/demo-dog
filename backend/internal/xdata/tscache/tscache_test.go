package tscache

import (
	"testing"
	"time"
)

func TestAddSnapshot(t *testing.T) {
	c := New(10, time.Minute)
	c.Add("a", 1)
	c.Add("a", 2)
	c.Add("a", 3)
	s := c.Snapshot("a")
	if len(s) != 3 || s[0].Value != 1 {
		t.Fatal("snap", s)
	}
}

func TestLatest(t *testing.T) {
	c := New(10, time.Minute)
	c.Add("a", 1)
	c.Add("a", 2)
	l, ok := c.Latest("a")
	if !ok || l.Value != 2 {
		t.Fatal("latest", l)
	}
}

func TestLatest_Empty(t *testing.T) {
	c := New(10, time.Minute)
	if _, ok := c.Latest("missing"); ok {
		t.Fatal("miss")
	}
}

func TestCapEvict(t *testing.T) {
	c := New(3, time.Minute)
	for i := 0; i < 10; i++ {
		c.Add("a", float64(i))
	}
	if len(c.Snapshot("a")) != 3 {
		t.Fatal("cap")
	}
}

func TestKeys(t *testing.T) {
	c := New(10, time.Minute)
	c.Add("a", 1)
	c.Add("b", 2)
	k := c.Keys()
	if len(k) != 2 {
		t.Fatal("keys", k)
	}
}

func TestClear(t *testing.T) {
	c := New(10, time.Minute)
	c.Add("a", 1)
	c.Clear("a")
	if _, ok := c.Latest("a"); ok {
		t.Fatal("clear")
	}
}
