package segmentx

import "testing"

func TestAdd(t *testing.T) {
	s := New()
	s.Add(0, 10)
	if !s.Contains(5) {
		t.Fatal("contains")
	}
}

func TestMerge(t *testing.T) {
	s := New()
	s.Add(0, 10)
	s.Add(5, 15)
	if !s.Contains(12) {
		t.Fatal("merge")
	}
	all := s.All()
	if len(all) != 1 || all[0].Lo != 0 || all[0].Hi != 15 {
		t.Fatal("merged", all)
	}
}

func TestMiss(t *testing.T) {
	s := New()
	s.Add(0, 10)
	if s.Contains(10) {
		t.Fatal("hi 应不含")
	}
}

func TestBadRange(t *testing.T) {
	s := New()
	s.Add(10, 5)
	if len(s.All()) != 0 {
		t.Fatal("应忽略")
	}
}

func TestAll(t *testing.T) {
	s := New()
	s.Add(0, 5)
	s.Add(10, 15)
	all := s.All()
	if len(all) != 2 {
		t.Fatal("all", all)
	}
}
