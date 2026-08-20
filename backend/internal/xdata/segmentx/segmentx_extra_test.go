package segmentx

import "testing"

func TestMergeBack(t *testing.T) {
	s := New()
	s.Add(0, 3)
	s.Add(5, 10)
	s.Add(2, 6)
	all := s.All()
	t.Logf("got %+v", all)
	if len(all) != 1 || all[0].Lo != 0 || all[0].Hi != 10 {
		t.Fatalf("merge failed: %+v", all)
	}
}

func TestContainsAfterMerge(t *testing.T) {
	s := New()
	s.Add(0, 3)
	s.Add(5, 10)
	s.Add(2, 6)
	if !s.Contains(0) || !s.Contains(9) || s.Contains(11) {
		t.Fatal("contains")
	}
}

func TestSortStable(t *testing.T) {
	s := New()
	s.Add(10, 20)
	s.Add(0, 5)
	s.Sort()
	all := s.All()
	if all[0].Lo != 0 {
		t.Fatal("sort", all)
	}
}

func TestEmptyAdd(t *testing.T) {
	s := New()
	s.Add(5, 5)
	if len(s.All()) != 0 {
		t.Fatal("empty")
	}
}
