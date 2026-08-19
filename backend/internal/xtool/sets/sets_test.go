package sets

import "testing"

func TestAdd(t *testing.T) {
	s := New()
	s.Add("a", "b")
	if s.Len() != 2 {
		t.Fatal("len")
	}
}

func TestRemove(t *testing.T) {
	s := New("a", "b")
	s.Remove("a")
	if s.Has("a") {
		t.Fatal("rm")
	}
}

func TestHas(t *testing.T) {
	s := New("x")
	if !s.Has("x") {
		t.Fatal("has")
	}
}

func TestSlice(t *testing.T) {
	s := New("a", "b")
	if len(s.Slice()) != 2 {
		t.Fatal("slice")
	}
}

func TestUnion(t *testing.T) {
	u := Union(New("a", "b"), New("b", "c"))
	if u.Len() != 3 || !u.Has("a") || !u.Has("c") {
		t.Fatal("union", u)
	}
}

func TestIntersect(t *testing.T) {
	x := Intersect(New("a", "b"), New("b", "c"))
	if x.Len() != 1 || !x.Has("b") {
		t.Fatal("intersect", x)
	}
}

func TestDiff(t *testing.T) {
	d := Diff(New("a", "b", "c"), New("b"))
	if d.Len() != 2 || !d.Has("a") || d.Has("b") {
		t.Fatal("diff", d)
	}
}
