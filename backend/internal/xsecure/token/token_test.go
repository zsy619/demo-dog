package token

import (
	"errors"
	"testing"
	"time"
)

func TestIssueConsume(t *testing.T) {
	s := NewMemStore()
	tk, err := Issue(s, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := Consume(s, tk); err != nil {
		t.Fatal(err)
	}
}

func TestConsumeOnce(t *testing.T) {
	s := NewMemStore()
	tk, _ := Issue(s, time.Minute)
	Consume(s, tk)
	if err := Consume(s, tk); !errors.Is(err, ErrUnknown) {
		t.Fatal("应 ErrUnknown")
	}
}

func TestConsume_Expired(t *testing.T) {
	s := NewMemStore()
	tk, _ := Issue(s, 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	if err := Consume(s, tk); !errors.Is(err, ErrExpired) {
		t.Fatal("应 ErrExpired")
	}
}

func TestPeek(t *testing.T) {
	s := NewMemStore()
	tk, _ := Issue(s, time.Minute)
	if err := Peek(s, tk); err != nil {
		t.Fatal("应通过")
	}
	// Peek 不消耗
	if err := Peek(s, tk); err != nil {
		t.Fatal("应仍可通过")
	}
}

func TestConsume_Unknown(t *testing.T) {
	s := NewMemStore()
	if err := Consume(s, "x"); !errors.Is(err, ErrUnknown) {
		t.Fatal("应 ErrUnknown")
	}
}

func TestSave_Time(t *testing.T) {
	s := NewMemStore()
	if err := s.Save("x", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Exists("x"); !ok {
		t.Fatal("exists")
	}
	s.Delete("x")
	if _, ok := s.Exists("x"); ok {
		t.Fatal("应被删")
	}
}
