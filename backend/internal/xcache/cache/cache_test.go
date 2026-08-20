package cache

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	c := New(Config{TTL: time.Minute})
	c.Set("k", "v")
	v, ok := c.Get("k")
	if !ok || v != "v" {
		t.Fatal("miss")
	}
}

func TestGet_Miss(t *testing.T) {
	c := New(Config{})
	if _, ok := c.Get("missing"); ok {
		t.Fatal("unexpected hit")
	}
}

func TestTTL(t *testing.T) {
	c := New(Config{TTL: 20 * time.Millisecond})
	c.Set("k", "v")
	time.Sleep(50 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected expiry")
	}
	s := c.Stats()
	if s.Expired == 0 {
		t.Fatal("expired counter")
	}
}

func TestDelete(t *testing.T) {
	c := New(Config{})
	c.Set("k", "v")
	c.Delete("k")
	if _, ok := c.Get("k"); ok {
		t.Fatal("delete failed")
	}
}

func TestEviction(t *testing.T) {
	c := New(Config{MaxItems: 2})
	c.Set("a", 1)
	time.Sleep(time.Millisecond)
	c.Set("b", 2)
	time.Sleep(time.Millisecond)
	c.Set("c", 3)
	if c.Len() > 2 {
		t.Fatalf("len: %d", c.Len())
	}
	if c.Stats().Evicted == 0 {
		t.Fatal("evicted counter")
	}
}

func TestGetOrLoad_FirstCall(t *testing.T) {
	c := New(Config{})
	calls := atomic.Uint64{}
	v, err := c.GetOrLoad("k", func() (any, error) {
		calls.Add(1)
		return "v", nil
	})
	if err != nil || v != "v" {
		t.Fatal(err, v)
	}
	if calls.Load() != 1 {
		t.Fatal("load called twice")
	}
}

func TestGetOrLoad_Cached(t *testing.T) {
	c := New(Config{})
	calls := atomic.Uint64{}
	load := func() (any, error) {
		calls.Add(1)
		return "v", nil
	}
	c.GetOrLoad("k", load)
	c.GetOrLoad("k", load)
	c.GetOrLoad("k", load)
	if calls.Load() != 1 {
		t.Fatalf("expected 1, got %d", calls.Load())
	}
}

func TestGetOrLoad_LoadErrorNotCached(t *testing.T) {
	c := New(Config{})
	calls := atomic.Uint64{}
	load := func() (any, error) {
		calls.Add(1)
		return nil, errTest
	}
	if _, err := c.GetOrLoad("k", load); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.GetOrLoad("k", load); err == nil {
		t.Fatal("expected error 2")
	}
	if calls.Load() != 2 {
		t.Fatalf("load: %d", calls.Load())
	}
}

func TestFlush(t *testing.T) {
	c := New(Config{})
	c.Set("a", 1)
	c.Set("b", 2)
	c.Flush()
	if c.Len() != 0 {
		t.Fatal("flush")
	}
}

var errTest = errTestSentinel{}

type errTestSentinel struct{}

func (errTestSentinel) Error() string { return "test" }
