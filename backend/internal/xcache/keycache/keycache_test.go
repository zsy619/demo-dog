package keycache

import (
	"testing"
	"time"
)

func TestPutGet(t *testing.T) {
	c := New(time.Minute)
	c.Put("a", "1")
	v, ok := c.Get("a")
	if !ok || v != "1" {
		t.Fatal("get")
	}
}

func TestExpire(t *testing.T) {
	c := New(10 * time.Millisecond)
	c.Put("a", "1")
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("a"); ok {
		t.Fatal("应过期")
	}
}

func TestNoTTL(t *testing.T) {
	c := New(0)
	c.Put("a", "1")
	if _, ok := c.Get("a"); !ok {
		t.Fatal("no ttl")
	}
}

func TestGC(t *testing.T) {
	c := New(10 * time.Millisecond)
	c.Put("a", "1")
	c.Put("b", "2")
	time.Sleep(30 * time.Millisecond)
	n := c.GC()
	if n != 2 {
		t.Fatal("gc", n)
	}
}

func TestDelete(t *testing.T) {
	c := New(time.Minute)
	c.Put("a", "1")
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("del")
	}
}
