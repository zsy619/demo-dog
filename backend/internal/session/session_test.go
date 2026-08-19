package session

import (
	"errors"
	"testing"
	"time"
)

func newStore() *Store {
	return New(time.Second, 0).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestCreate(t *testing.T) {
	s := newStore()
	sess, err := s.Create("alice", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Subject != "alice" || sess.Tenant != "t1" || sess.ID == "" {
		t.Fatal("bad session")
	}
}

func TestGet_Slides(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second, 0).WithTime(func() time.Time { return now })
	sess, _ := s.Create("alice", "t1")
	now = now.Add(500 * time.Millisecond)
	g, err := s.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g.ExpiresAt != now.Add(time.Second) {
		t.Fatal("should slide")
	}
}

func TestGet_Expired(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second, 0).WithTime(func() time.Time { return now })
	sess, _ := s.Create("alice", "t1")
	now = now.Add(2 * time.Second)
	if _, err := s.Get(sess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestGet_Missing(t *testing.T) {
	s := newStore()
	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestPeek_NoSlide(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second, 0).WithTime(func() time.Time { return now })
	sess, _ := s.Create("alice", "t1")
	now = now.Add(500 * time.Millisecond)
	p, err := s.Peek(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.ExpiresAt != sess.ExpiresAt {
		t.Fatal("peek should not slide")
	}
}

func TestDelete(t *testing.T) {
	s := newStore()
	sess, _ := s.Create("alice", "t1")
	s.Delete(sess.ID)
	if _, err := s.Get(sess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestSet(t *testing.T) {
	s := newStore()
	sess, _ := s.Create("alice", "t1")
	if err := s.Set(sess.ID, "role", "admin"); err != nil {
		t.Fatal(err)
	}
	g, _ := s.Peek(sess.ID)
	if g.Data["role"] != "admin" {
		t.Fatal("data")
	}
}

func TestSet_Missing(t *testing.T) {
	s := newStore()
	if err := s.Set("missing", "k", "v"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestCleanup(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second, 0).WithTime(func() time.Time { return now })
	s.Create("a", "t")
	s.Create("b", "t")
	now = now.Add(2 * time.Second)
	n := s.Cleanup()
	if n != 2 {
		t.Fatalf("cleanup: %d", n)
	}
}

func TestMaxItems(t *testing.T) {
	s := New(time.Second, 2)
	s.Create("a", "t")
	s.Create("b", "t")
	s.Create("c", "t")
	if s.Len() != 2 {
		t.Fatalf("max: %d", s.Len())
	}
}

func TestLen(t *testing.T) {
	s := newStore()
	if s.Len() != 0 {
		t.Fatal("empty")
	}
	s.Create("a", "t")
	if s.Len() != 1 {
		t.Fatal("after create")
	}
}

func TestNewID_Unique(t *testing.T) {
	s := newStore()
	sess1, _ := s.Create("a", "t1")
	sess2, _ := s.Create("a", "t1")
	if sess1.ID == sess2.ID {
		t.Fatal("ids should be unique")
	}
}
