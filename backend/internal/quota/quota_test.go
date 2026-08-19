package quota

import (
	"testing"
	"time"
)

func TestBucket_Allow(t *testing.T) {
	b := &Bucket{Capacity: 2, Refill: 1, Tokens: 2, LastRefil: time.Now()}
	if !b.Allow(time.Now()) {
		t.Fatal("first")
	}
	if !b.Allow(time.Now()) {
		t.Fatal("second")
	}
	if b.Allow(time.Now()) {
		t.Fatal("third should be denied")
	}
}

func TestBucket_Refill(t *testing.T) {
	now := time.Now()
	b := &Bucket{Capacity: 10, Refill: 1, Tokens: 0, LastRefil: now}
	if b.Allow(now) {
		t.Fatal("zero tokens")
	}
	later := now.Add(2 * time.Second)
	if !b.Allow(later) {
		t.Fatal("refilled")
	}
}

func TestBucket_RefillCapped(t *testing.T) {
	now := time.Now()
	b := &Bucket{Capacity: 5, Refill: 100, Tokens: 5, LastRefil: now}
	later := now.Add(1 * time.Hour)
	// Burn all 5 quickly.
	for i := 0; i < 5; i++ {
		if !b.Allow(later) {
			t.Fatal("should allow")
		}
	}
	if b.Allow(later) {
		t.Fatal("should be capped")
	}
}

func TestManager_SetAndAllow(t *testing.T) {
	m := NewManager(100, 10)
	m.Set("acme", 5, 1)
	for i := 0; i < 5; i++ {
		ok, err := m.Allow("acme")
		if err != nil || !ok {
			t.Fatal(err, ok)
		}
	}
	ok, _ := m.Allow("acme")
	if ok {
		t.Fatal("should be denied")
	}
}

func TestManager_DefaultFallback(t *testing.T) {
	m := NewManager(2, 0.1)
	if ok, err := m.Allow("unknown"); err != nil || !ok {
		t.Fatal(err, ok)
	}
}

func TestManager_NotConfigured(t *testing.T) {
	m := NewManager(0, 0)
	if _, err := m.Allow("x"); err != ErrTenantNotConfigured {
		t.Fatalf("expected ErrTenantNotConfigured, got %v", err)
	}
}

func TestManager_Remove(t *testing.T) {
	m := NewManager(10, 1)
	m.Set("acme", 1, 0)
	m.Remove("acme")
	// Now falls back to default.
	ok, _ := m.Allow("acme")
	if !ok {
		t.Fatal("default fallback")
	}
}

func TestManager_Tokens(t *testing.T) {
	m := NewManager(10, 1)
	m.Set("acme", 5, 0.5)
	tok := m.Tokens("acme")
	if tok < 0 || tok > 5 {
		t.Fatalf("tokens: %v", tok)
	}
}

func TestManager_Snapshot(t *testing.T) {
	m := NewManager(1, 1)
	m.Set("acme", 2, 0.5)
	m.Set("globex", 3, 1)
	snap := m.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("snapshot: %d", len(snap))
	}
}
