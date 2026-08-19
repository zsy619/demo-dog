package softdel

import (
	"errors"
	"testing"
	"time"
)

func newStore() *Store {
	return New(time.Second).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestPutGet(t *testing.T) {
	s := newStore()
	s.Put("a", []byte("x"))
	r, err := s.Get("a")
	if err != nil || r.Data[0] != 'x' {
		t.Fatal(err)
	}
}

func TestGet_Missing(t *testing.T) {
	s := newStore()
	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	s := newStore()
	s.Put("a", []byte("x"))
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestDelete_AlreadyDeleted(t *testing.T) {
	s := newStore()
	s.Put("a", []byte("x"))
	s.Delete("a")
	if err := s.Delete("a"); !errors.Is(err, ErrAlreadyDeleted) {
		t.Fatal(err)
	}
}

func TestDelete_Missing(t *testing.T) {
	s := newStore()
	if err := s.Delete("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestRestore(t *testing.T) {
	s := newStore()
	s.Put("a", []byte("x"))
	s.Delete("a")
	if err := s.Restore("a"); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Get("a")
	if r.Data[0] != 'x' {
		t.Fatal("restore")
	}
}

func TestRestore_Missing(t *testing.T) {
	s := newStore()
	if err := s.Restore("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestRestore_Live(t *testing.T) {
	s := newStore()
	s.Put("a", []byte("x"))
	if err := s.Restore("a"); err != nil {
		t.Fatal(err)
	}
}

func TestReclaim(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second).WithTime(func() time.Time { return now })
	s.Put("a", []byte("x"))
	s.Put("b", []byte("y"))
	s.Delete("a")
	now = now.Add(2 * time.Second)
	n := s.Reclaim()
	if n != 1 {
		t.Fatal("reclaim")
	}
	if s.Len() != 1 {
		t.Fatal("len")
	}
}

func TestReclaim_NotExpired(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Second).WithTime(func() time.Time { return now })
	s.Put("a", []byte("x"))
	s.Delete("a")
	n := s.Reclaim()
	if n != 0 {
		t.Fatal("should not reclaim")
	}
}

func TestList(t *testing.T) {
	s := newStore()
	s.Put("a", []byte("x"))
	s.Put("b", []byte("y"))
	s.Delete("a")
	list := s.List()
	if len(list) != 1 || list[0].ID != "b" {
		t.Fatal("list")
	}
}

func TestLen(t *testing.T) {
	s := newStore()
	s.Put("a", []byte("x"))
	s.Delete("a")
	if s.Len() != 1 {
		t.Fatal("len")
	}
	if s.Live() != 0 {
		t.Fatal("live")
	}
}
