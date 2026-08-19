package bitset

import "testing"

func TestSetTest(t *testing.T) {
	b := New()
	b.Set(3)
	b.Set(100)
	if !b.Test(3) || !b.Test(100) {
		t.Fatal("set")
	}
	if b.Test(4) {
		t.Fatal("not set")
	}
}

func TestClear(t *testing.T) {
	b := New()
	b.Set(5)
	b.Clear(5)
	if b.Test(5) {
		t.Fatal("clear")
	}
}

func TestClear_OutOfRange(t *testing.T) {
	b := New()
	b.Clear(999) // 不 panic
}

func TestCount(t *testing.T) {
	b := New()
	b.Set(0)
	b.Set(1)
	b.Set(64)
	if b.Count() != 3 {
		t.Fatal("count")
	}
}

func TestUnion(t *testing.T) {
	a := New()
	c := New()
	a.Set(1)
	c.Set(2)
	a.Union(c)
	if !a.Test(1) || !a.Test(2) {
		t.Fatal("union")
	}
}

func TestIntersection(t *testing.T) {
	a := New()
	c := New()
	a.Set(1)
	a.Set(2)
	c.Set(2)
	c.Set(3)
	a.Intersection(c)
	if a.Test(1) || !a.Test(2) || a.Test(3) {
		t.Fatal("inter")
	}
}

func TestIndices(t *testing.T) {
	b := New()
	b.Set(1)
	b.Set(3)
	idx := b.Indices()
	if len(idx) != 2 || idx[0] != 1 || idx[1] != 3 {
		t.Fatal("indices")
	}
}
