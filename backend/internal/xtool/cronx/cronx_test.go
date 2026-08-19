package cronx

import (
	"testing"
	"time"
)

func TestParse_EveryMinute(t *testing.T) {
	s, err := Parse("* * * * *")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Match(time.Now()) {
		t.Fatal("应匹配")
	}
}

func TestMatch_Hour(t *testing.T) {
	s, _ := Parse("0 12 * * *")
	if !s.Match(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("12:00")
	}
	if s.Match(time.Date(2025, 1, 1, 13, 0, 0, 0, time.UTC)) {
		t.Fatal("13:00 应拒")
	}
}

func TestParse_BadField(t *testing.T) {
	if _, err := Parse("a * * * *"); err == nil {
		t.Fatal("应报错")
	}
}

func TestParse_BadCount(t *testing.T) {
	if _, err := Parse("* *"); err == nil {
		t.Fatal("应报错")
	}
}

func TestMatch_Range(t *testing.T) {
	s, _ := Parse("0 9-17 * * *")
	if !s.Match(time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("range")
	}
}

func TestMatch_Step(t *testing.T) {
	s, _ := Parse("*/15 * * * *")
	if !s.Match(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("step 0")
	}
	if !s.Match(time.Date(2025, 1, 1, 0, 15, 0, 0, time.UTC)) {
		t.Fatal("step 15")
	}
	if s.Match(time.Date(2025, 1, 1, 0, 7, 0, 0, time.UTC)) {
		t.Fatal("step 7 应拒")
	}
}

func TestNext(t *testing.T) {
	s, _ := Parse("30 9 * * *")
	t1 := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)
	n := s.Next(t1)
	if n.Hour() != 9 || n.Minute() != 30 {
		t.Fatal("next")
	}
}
