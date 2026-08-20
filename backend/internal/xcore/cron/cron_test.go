package cron

import (
	"testing"
	"time"
)

func TestParse_BadFields(t *testing.T) {
	if _, err := Parse("*"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMatch_AllStars(t *testing.T) {
	s, err := Parse("* * * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC)
	if !s.Match(now) {
		t.Fatal("should match")
	}
}

func TestMatch_MinuteOnly(t *testing.T) {
	s, _ := Parse("30 * * * *")
	now := time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC)
	if !s.Match(now) {
		t.Fatal("should match minute")
	}
	now2 := now.Add(time.Minute)
	if s.Match(now2) {
		t.Fatal("should not match minute 31")
	}
}

func TestMatch_OutOfRange(t *testing.T) {
	if _, err := Parse("60 * * * *"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMatch_Range(t *testing.T) {
	s, err := Parse("0 9-17 * * *")
	if err != nil {
		t.Fatal(err)
	}
	for h := 9; h <= 17; h++ {
		t0 := time.Date(2026, 1, 1, h, 0, 0, 0, time.UTC)
		if !s.Match(t0) {
			t.Fatalf("hour %d should match", h)
		}
	}
}

func TestMatch_Step(t *testing.T) {
	s, _ := Parse("*/15 * * * *")
	for m := 0; m < 60; m++ {
		t0 := time.Date(2026, 1, 1, 0, m, 0, 0, time.UTC)
		want := m%15 == 0
		if s.Match(t0) != want {
			t.Fatalf("minute %d: got %v want %v", m, !want, want)
		}
	}
}

func TestMatch_List(t *testing.T) {
	s, _ := Parse("0,30 * * * *")
	for _, m := range []int{0, 30, 60} {
		t0 := time.Date(2026, 1, 1, 0, m%60, 0, 0, time.UTC)
		if !s.Match(t0) {
			t.Fatalf("minute %d should match", m)
		}
	}
}

func TestNext(t *testing.T) {
	s, _ := Parse("0 12 * * *")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nxt := s.Next(now)
	want := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if !nxt.Equal(want) {
		t.Fatalf("next: %v want %v", nxt, want)
	}
}

func TestScheduler_Add(t *testing.T) {
	s := New()
	if err := s.Add("a", "* * * * *", func(time.Time) {}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("b", "bad expr", func(time.Time) {}); err == nil {
		t.Fatal("expected error")
	}
	if len(s.List()) != 1 {
		t.Fatal("list")
	}
}

func TestScheduler_Tick(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC)
	s := New().WithTime(func() time.Time { return now })
	called := 0
	s.Add("a", "30 * * * *", func(t time.Time) { called++ })
	s.Add("b", "0 * * * *", func(t time.Time) { called++ })
	n := s.Tick()
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
	if called != 1 {
		t.Fatalf("called: %d", called)
	}
	// 时间相同，不重复触发。
	if n2 := s.Tick(); n2 != 0 {
		t.Fatalf("expected 0 on second tick, got %d", n2)
	}
}
