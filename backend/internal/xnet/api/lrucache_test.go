package api

import (
	"fmt"
	"testing"
	"time"
)

func TestLRUCache_BasicGetSet(t *testing.T) {
	c := NewLRUCache(10, 0, 0)
	c.Set("k1", []byte("v1"))
	got, ok := c.Get("k1")
	if !ok || string(got) != "v1" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("missing key should not be found")
	}
}

func TestLRUCache_EvictsWhenFull(t *testing.T) {
	c := NewLRUCache(2, 0, 0)
	c.Set("a", []byte("1"))
	c.Set("b", []byte("2"))
	c.Set("c", []byte("3"))
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("b should still be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("c should still be present")
	}
}

func TestLRUCache_LRUOrder(t *testing.T) {
	c := NewLRUCache(2, 0, 0)
	c.Set("a", []byte("1"))
	c.Set("b", []byte("2"))
	c.Get("a")
	c.Set("c", []byte("3"))
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted")
	}
}

func TestLRUCache_TTL(t *testing.T) {
	c := NewLRUCache(10, 0, 30*time.Millisecond)
	c.Set("k", []byte("v"))
	if _, ok := c.Get("k"); !ok {
		t.Fatal("fresh entry should hit")
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expired entry should miss")
	}
	if c.Len() != 0 {
		t.Fatalf("expected empty after expiry, got %d", c.Len())
	}
}

func TestLRUCache_MaxBytes(t *testing.T) {
	c := NewLRUCache(100, 10, 0)
	c.Set("a", []byte("12345"))
	c.Set("b", []byte("12345"))
	c.Set("c", []byte("12345"))
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted by byte budget")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("b should remain")
	}
}

func TestLRUCache_Delete(t *testing.T) {
	c := NewLRUCache(10, 0, 0)
	c.Set("k", []byte("v"))
	c.Delete("k")
	if _, ok := c.Get("k"); ok {
		t.Fatal("deleted key should not be found")
	}
	c.Delete("never-existed")
}

func TestLRUCache_ValueImmutability(t *testing.T) {
	c := NewLRUCache(10, 0, 0)
	v := []byte("hello")
	c.Set("k", v)
	v[0] = 'X'
	got, _ := c.Get("k")
	if string(got) != "hello" {
		t.Fatalf("cache value was mutated: %s", got)
	}
}

func TestLRUCache_Stats(t *testing.T) {
	c := NewLRUCache(2, 0, 0)
	c.Set("a", []byte("1"))
	c.Get("a")
	c.Get("missing")
	c.Set("b", []byte("2"))
	c.Set("c", []byte("3"))
	s := c.Stats()
	if s.Hits != 1 || s.Misses != 1 {
		t.Fatalf("hits=%d misses=%d", s.Hits, s.Misses)
	}
	if s.Evictions == 0 {
		t.Fatal("expected at least one eviction")
	}
}

func TestLRUCache_Concurrent(t *testing.T) {
	c := NewLRUCache(100, 0, 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			c.Set(fmt.Sprintf("k%d", i%50), []byte("v"))
		}
	}()
	for i := 0; i < 1000; i++ {
		c.Get(fmt.Sprintf("k%d", i%50))
	}
	<-done
}
