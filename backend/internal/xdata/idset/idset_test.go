package idset

import "testing"

func TestAddHas(t *testing.T) {
	s := New()
	s.Add(10)
	if !s.Has(10) {
		t.Fatal("add")
	}
}

func TestRemove(t *testing.T) {
	s := New()
	s.Add(1)
	s.Remove(1)
	if s.Has(1) {
		t.Fatal("rm")
	}
}

func TestMinMax(t *testing.T) {
	s := New()
	s.Add(5)
	s.Add(10)
	s.Add(2)
	mn, _ := s.Min()
	mx, _ := s.Max()
	if mn != 2 || mx != 10 {
		t.Fatal("mm", mn, mx)
	}
}

func TestEmpty(t *testing.T) {
	s := New()
	if _, ok := s.Min(); ok {
		t.Fatal("min")
	}
	if _, ok := s.Max(); ok {
		t.Fatal("max")
	}
}

func TestUnion(t *testing.T) {
	a := New()
	a.Add(1)
	a.Add(2)
	b := New()
	b.Add(2)
	b.Add(3)
	u := a.Union(b)
	if u.Len() != 3 {
		t.Fatal("union", u.Len())
	}
}

func TestClear(t *testing.T) {
	s := New()
	s.Add(1)
	s.Clear()
	if s.Len() != 0 {
		t.Fatal("clear")
	}
}
