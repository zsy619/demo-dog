package ratelimit

import (
	"errors"
	"testing"
	"time"
)

func TestTokenBucket_BurstAndRefill(t *testing.T) {
	fakeNow := time.Unix(1700000000, 0)
	l := New(Settings{Capacity: 5, RefillPerSec: 1, Now: func() time.Time { return fakeNow }})
	for i := 0; i < 5; i++ {
		if err := l.AllowTokenBucket("k"); err != nil {
			t.Fatalf("burst %d: %v", i, err)
		}
	}
	if err := l.AllowTokenBucket("k"); !errors.Is(err, ErrLimited) {
		t.Fatal("expected limited after burst")
	}
	// Advance 3 seconds; refill = 3 tokens.
	fakeNow = fakeNow.Add(3 * time.Second)
	for i := 0; i < 3; i++ {
		if err := l.AllowTokenBucket("k"); err != nil {
			t.Fatalf("refill %d: %v", i, err)
		}
	}
	if err := l.AllowTokenBucket("k"); !errors.Is(err, ErrLimited) {
		t.Fatal("expected limited again")
	}
}

func TestTokenBucket_DefaultSettings(t *testing.T) {
	l := New(Settings{})
	// Should allow at least the default capacity.
	for i := 0; i < 100; i++ {
		if err := l.AllowTokenBucket("k"); err != nil {
			t.Fatalf("default burst %d: %v", i, err)
		}
	}
}

func TestTokenBucket_PerKey(t *testing.T) {
	l := New(Settings{Capacity: 1, RefillPerSec: 0})
	if err := l.AllowTokenBucket("a"); err != nil {
		t.Fatal(err)
	}
	if err := l.AllowTokenBucket("b"); err != nil {
		t.Fatal("b should have its own bucket")
	}
	if err := l.AllowTokenBucket("a"); !errors.Is(err, ErrLimited) {
		t.Fatal("a should now be limited")
	}
}

func TestTokenBucket_Tokens(t *testing.T) {
	fakeNow := time.Unix(1700000000, 0)
	l := New(Settings{Capacity: 10, RefillPerSec: 0, Now: func() time.Time { return fakeNow }})
	if v := l.Tokens("k"); v != 10 {
		t.Fatalf("initial tokens: %v", v)
	}
	l.AllowTokenBucket("k")
	if v := l.Tokens("k"); v != 9 {
		t.Fatalf("after one: %v", v)
	}
}

func TestLeakyBucket_Smooth(t *testing.T) {
	fakeNow := time.Unix(1700000000, 0)
	l := New(Settings{Capacity: 1, LeakPerSec: 1, Now: func() time.Time { return fakeNow }})
	// First request goes through (bucket capacity 1).
	if err := l.AllowLeakyBucket("k"); err != nil {
		t.Fatal(err)
	}
	// Second one is queued (level=1, fill = 1).
	if err := l.AllowLeakyBucket("k"); !errors.Is(err, ErrLimited) {
		t.Fatal("second should be limited")
	}
	// 1 second later, the level has leaked down to 0.
	fakeNow = fakeNow.Add(2 * time.Second)
	if err := l.AllowLeakyBucket("k"); err != nil {
		t.Fatal(err)
	}
}

func TestLeakyBucket_PerKey(t *testing.T) {
	l := New(Settings{Capacity: 1, LeakPerSec: 0})
	if err := l.AllowLeakyBucket("a"); err != nil {
		t.Fatal(err)
	}
	if err := l.AllowLeakyBucket("b"); err != nil {
		t.Fatal("b independent")
	}
}

func TestReset(t *testing.T) {
	l := New(Settings{Capacity: 1, RefillPerSec: 0})
	l.AllowTokenBucket("k")
	l.Reset("k")
	if err := l.AllowTokenBucket("k"); err != nil {
		t.Fatal("after reset should allow")
	}
}

func TestSnapshot(t *testing.T) {
	l := New(Settings{Capacity: 5, RefillPerSec: 1})
	l.AllowTokenBucket("a")
	l.AllowLeakyBucket("b")
	s := l.Snapshot()
	if s.Shards != 2 {
		t.Fatalf("shards: %d", s.Shards)
	}
	if len(s.TokenKeys) != 1 || len(s.LeakKeys) != 1 {
		t.Fatalf("counts: %+v", s)
	}
}

func TestMaxShards(t *testing.T) {
	l := New(Settings{Capacity: 1, RefillPerSec: 0, MaxShards: 2})
	l.AllowTokenBucket("a")
	l.AllowTokenBucket("b")
	if err := l.AllowTokenBucket("c"); !errors.Is(err, ErrLimited) {
		t.Fatal("should refuse to create new shard at cap")
	}
}

func TestConcurrentSafe(t *testing.T) {
	l := New(Settings{Capacity: 1000, RefillPerSec: 1000})
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = l.AllowTokenBucket("k")
			}
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
