package secretrot

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func newStore() *Store {
	return New(time.Hour).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestSetGet(t *testing.T) {
	s := newStore()
	s.Set("t1", []byte("hello"))
	v, err := s.Get("t1")
	if err != nil || !bytes.Equal(v, []byte("hello")) {
		t.Fatal(err)
	}
}

func TestGet_Missing(t *testing.T) {
	s := newStore()
	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestRotate(t *testing.T) {
	s := newStore()
	s.Set("t1", []byte("a"))
	if err := s.Rotate("t1"); err != nil {
		t.Fatal(err)
	}
	if s.Rotations("t1") != 1 {
		t.Fatal("rotations")
	}
}

func TestRotate_Missing(t *testing.T) {
	s := newStore()
	if err := s.Rotate("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestGet_RotateOnExpire(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New(time.Hour).WithTime(func() time.Time { return now })
	s.Set("t1", []byte("a"))
	v, _ := s.Get("t1")
	if !bytes.Equal(v, []byte("a")) {
		t.Fatal("first")
	}
	now = now.Add(2 * time.Hour)
	v, _ = s.Get("t1")
	if bytes.Equal(v, []byte("a")) {
		t.Fatal("should rotate")
	}
	if s.Rotations("t1") != 1 {
		t.Fatal("count")
	}
}

func TestDelete(t *testing.T) {
	s := newStore()
	s.Set("t1", []byte("a"))
	s.Delete("t1")
	if _, err := s.Get("t1"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestSnapshot(t *testing.T) {
	s := newStore()
	s.Set("a", []byte("x"))
	s.Set("b", []byte("y"))
	snap := s.Snapshot()
	if len(snap) != 2 {
		t.Fatal("snap")
	}
}

func TestGetCopy(t *testing.T) {
	s := newStore()
	s.Set("t", []byte("hello"))
	v, _ := s.Get("t")
	v[0] = 'X'
	v2, _ := s.Get("t")
	if v2[0] != 'h' {
		t.Fatal("isolation")
	}
}
