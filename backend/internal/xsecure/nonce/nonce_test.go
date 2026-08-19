package nonce

import (
	"errors"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	n := Generate()
	if len(n) != 32 {
		t.Fatal("len")
	}
}

func TestCheck_FirstOK(t *testing.T) {
	s := New(time.Minute)
	if err := s.Check("abc", time.Now().Unix()); err != nil {
		t.Fatal("first")
	}
}

func TestCheck_Replay(t *testing.T) {
	s := New(time.Minute)
	ts := time.Now().Unix()
	s.Check("abc", ts)
	if err := s.Check("abc", ts); !errors.Is(err, ErrReplay) {
		t.Fatal("应 replay")
	}
}

func TestCheck_Expired(t *testing.T) {
	s := New(time.Minute)
	if err := s.Check("x", time.Now().Add(-time.Hour).Unix()); !errors.Is(err, ErrReplay) {
		t.Fatal("应过期")
	}
}

func TestMark(t *testing.T) {
	s := New(time.Minute)
	s.Mark("y")
	if s.Len() != 1 {
		t.Fatal("len")
	}
}

func TestSweep(t *testing.T) {
	s := New(time.Second)
	s.SetNow(func() time.Time { return time.Unix(0, 0) })
	s.Mark("old")
	s.SetNow(func() time.Time { return time.Unix(60, 0) })
	if s.Len() != 0 {
		t.Fatal("sweep")
	}
}
