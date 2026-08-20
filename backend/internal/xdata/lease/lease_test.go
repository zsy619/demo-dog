package lease

import (
	"errors"
	"testing"
	"time"
)

func newMgr() *Manager {
	return New(time.Second).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestAcquire_Free(t *testing.T) {
	m := newMgr()
	l, err := m.Acquire("a", "h1")
	if err != nil {
		t.Fatal(err)
	}
	if l.Holder != "h1" || l.ID == "" {
		t.Fatal(l)
	}
}

func TestAcquire_Contention(t *testing.T) {
	m := newMgr()
	m.Acquire("a", "h1")
	if _, err := m.Acquire("a", "h2"); !errors.Is(err, ErrHeld) {
		t.Fatalf("expected ErrHeld, got %v", err)
	}
}

func TestAcquire_AfterExpiry(t *testing.T) {
	now := time.Unix(1700000000, 0)
	m := New(time.Second).WithTime(func() time.Time { return now })
	m.Acquire("a", "h1")
	now = now.Add(2 * time.Second)
	l, err := m.Acquire("a", "h2")
	if err != nil {
		t.Fatal(err)
	}
	if l.Holder != "h2" {
		t.Fatal("should switch holder")
	}
}

func TestRenew(t *testing.T) {
	now := time.Unix(1700000000, 0)
	m := New(time.Second).WithTime(func() time.Time { return now })
	m.Acquire("a", "h1")
	now = now.Add(500 * time.Millisecond)
	l, err := m.Renew("a", "h1")
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(time.Second)
	if !l.ExpiresAt.Equal(want) {
		t.Fatalf("expiry: got %v want %v", l.ExpiresAt, want)
	}
}

func TestRenew_WrongHolder(t *testing.T) {
	m := newMgr()
	m.Acquire("a", "h1")
	if _, err := m.Renew("a", "h2"); !errors.Is(err, ErrHeld) {
		t.Fatal(err)
	}
}

func TestRelease(t *testing.T) {
	m := newMgr()
	m.Acquire("a", "h1")
	if err := m.Release("a", "h1"); err != nil {
		t.Fatal(err)
	}
	if m.Get("a") != nil {
		t.Fatal("should be gone")
	}
}

func TestRelease_WrongHolder(t *testing.T) {
	m := newMgr()
	m.Acquire("a", "h1")
	if err := m.Release("a", "h2"); !errors.Is(err, ErrHeld) {
		t.Fatal(err)
	}
}

func TestRelease_Idempotent(t *testing.T) {
	m := newMgr()
	if err := m.Release("missing", "h1"); err != nil {
		t.Fatal("should be no-op")
	}
}

func TestGet_Missing(t *testing.T) {
	m := newMgr()
	if l := m.Get("missing"); l != nil {
		t.Fatal(l)
	}
}

func TestSweep(t *testing.T) {
	now := time.Unix(1700000000, 0)
	m := New(time.Second).WithTime(func() time.Time { return now })
	m.Acquire("a", "h1")
	m.Acquire("b", "h2")
	now = now.Add(2 * time.Second)
	n := m.Sweep()
	if n != 2 {
		t.Fatalf("sweep: %d", n)
	}
	if m.Active() != 0 {
		t.Fatal("active")
	}
}

func TestActive(t *testing.T) {
	now := time.Unix(1700000000, 0)
	m := New(time.Second).WithTime(func() time.Time { return now })
	m.Acquire("a", "h1")
	if m.Active() != 1 {
		t.Fatal("active")
	}
	now = now.Add(2 * time.Second)
	if m.Active() != 0 {
		t.Fatal("after expiry")
	}
}

func TestNames(t *testing.T) {
	m := newMgr()
	m.Acquire("a", "h1")
	m.Acquire("b", "h2")
	names := m.Names()
	if len(names) != 2 {
		t.Fatalf("names: %v", names)
	}
}
