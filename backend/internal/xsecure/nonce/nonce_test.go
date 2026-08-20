package nonce

import (
	"errors"
	"testing"
	"time"
)

func newStore() *Store {
	return New(time.Second, 0).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestCheck_First(t *testing.T) {
	s := newStore()
	if err := s.Check("t", "n1", time.Unix(1700000000, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestCheck_Replay(t *testing.T) {
	s := newStore()
	now := time.Unix(1700000000, 0)
	if err := s.Check("t", "n1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Check("t", "n1", now); !errors.Is(err, ErrReplay) {
		t.Fatal(err)
	}
}

func TestCheck_EmptyNonce(t *testing.T) {
	s := newStore()
	if err := s.Check("t", "", time.Unix(1700000000, 0)); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheck_Expired(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second, 0).WithTime(func() time.Time { return now })
	if err := s.Check("t", "n1", now); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := s.Check("t", "n1", now); err != nil {
		t.Fatal("should not be replay")
	}
}

func TestCheck_TenantIsolation(t *testing.T) {
	s := newStore()
	if err := s.Check("t1", "n1", time.Unix(1700000000, 0)); err != nil {
		t.Fatal(err)
	}
	if err := s.Check("t2", "n1", time.Unix(1700000000, 0)); err != nil {
		t.Fatal("different tenant OK")
	}
}

func TestForget(t *testing.T) {
	s := newStore()
	now := time.Unix(1700000000, 0)
	s.Check("t", "n1", now)
	s.Forget("t", "n1")
	if err := s.Check("t", "n1", now); err != nil {
		t.Fatal("forgotten OK")
	}
}

func TestCleanup(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second, 0).WithTime(func() time.Time { return now })
	s.Check("t", "n1", now)
	s.Check("t", "n2", now)
	now = now.Add(2 * time.Second)
	if n := s.Cleanup(); n != 2 {
		t.Fatal("cleanup")
	}
}

func TestMaxItems(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second, 2).WithTime(func() time.Time { return now })
	s.Check("t", "n1", now)
	s.Check("t", "n2", now)
	s.Check("t", "n3", now)
	if s.Len() != 2 {
		t.Fatal("cap")
	}
}

func TestLen(t *testing.T) {
	s := newStore()
	if s.Len() != 0 {
		t.Fatal("empty")
	}
	s.Check("t", "n1", time.Unix(1700000000, 0))
	if s.Len() != 1 {
		t.Fatal("after")
	}
}
