package window

import (
	"testing"
	"time"
)

func TestPutGet(t *testing.T) {
	c := New(4, time.Minute)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestExpiry(t *testing.T) {
	c := New(4, 30*time.Millisecond)
	c.Put("a", 1)
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("a"); ok {
		t.Fatal("应过期")
	}
}

func TestSweep(t *testing.T) {
	c := New(4, 30*time.Millisecond)
	c.Put("a", 1)
	time.Sleep(60 * time.Millisecond)
	n := c.Sweep()
	if n != 1 {
		t.Fatal("sweep")
	}
}

func TestEviction(t *testing.T) {
	c := New(2, time.Minute)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 应淘汰")
	}
}

func TestClear(t *testing.T) {
	c := New(4, time.Minute)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}
