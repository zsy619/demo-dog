package ttlcache

import (
	"testing"
	"time"
)

func TestPutGet(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestExpiry(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	c.PutTTL("a", 1, 30*time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("a"); ok {
		t.Fatal("应过期")
	}
}

func TestDelete(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	c.Put("a", 1)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestLen(t *testing.T) {
	c := New(time.Minute)
	defer c.Close()
	c.Put("a", 1)
	if c.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClose(t *testing.T) {
	c := New(time.Minute)
	c.Close()
	c.Close() // 幂等
}
