package idset

import "testing"

func TestRemoveMinStale(t *testing.T) {
	s := New()
	s.Add(1)
	s.Add(5)
	s.Remove(1)
	min, _ := s.Min()
	if min != 5 {
		t.Fatalf("min 应为 5，得到 %d", min)
	}
}

func TestRemoveMaxStale(t *testing.T) {
	s := New()
	s.Add(1)
	s.Add(5)
	s.Remove(5)
	max, _ := s.Max()
	if max != 1 {
		t.Fatalf("max 应为 1，得到 %d", max)
	}
}
