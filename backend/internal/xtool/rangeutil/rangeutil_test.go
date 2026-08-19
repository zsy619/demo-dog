package rangeutil

import "testing"

func TestContains(t *testing.T) {
	r := Range{Start: 1, End: 5}
	if !r.Contains(3) || r.Contains(6) {
		t.Fatal("contains")
	}
}

func TestOverlaps(t *testing.T) {
	a := Range{1, 5}
	b := Range{3, 7}
	if !a.Overlaps(b) {
		t.Fatal("overlaps")
	}
}

func TestOverlaps_No(t *testing.T) {
	a := Range{1, 5}
	b := Range{6, 9}
	if a.Overlaps(b) {
		t.Fatal("no overlap")
	}
}

func TestMerge(t *testing.T) {
	out := Merge([]Range{{1, 3}, {2, 5}, {8, 10}, {9, 12}})
	if len(out) != 2 {
		t.Fatal("len", len(out))
	}
	if out[0].Start != 1 || out[0].End != 5 {
		t.Fatal("first")
	}
	if out[1].Start != 8 || out[1].End != 12 {
		t.Fatal("second")
	}
}

func TestMerge_Empty(t *testing.T) {
	if out := Merge(nil); out != nil {
		t.Fatal("empty")
	}
}

func TestMerge_Adjacent(t *testing.T) {
	out := Merge([]Range{{1, 3}, {4, 6}})
	if len(out) != 1 || out[0].End != 6 {
		t.Fatal("adj")
	}
}

func TestSubtract(t *testing.T) {
	r := Subtract(Range{1, 10}, Range{3, 7})
	if len(r) != 2 || r[0].Start != 1 || r[0].End != 2 || r[1].Start != 8 || r[1].End != 10 {
		t.Fatal("sub", r)
	}
}

func TestSubtract_NoOverlap(t *testing.T) {
	r := Subtract(Range{1, 5}, Range{6, 10})
	if len(r) != 1 {
		t.Fatal("no")
	}
}

func TestLength(t *testing.T) {
	r := Range{1, 5}
	if r.Length() != 5 {
		t.Fatal("len")
	}
}
