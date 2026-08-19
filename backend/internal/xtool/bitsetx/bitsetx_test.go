package bitsetx

import "testing"

func TestSetGet(t *testing.T) {
	b := New(128)
	b.Set(7)
	if !b.Get(7) {
		t.Fatal("set")
	}
}

func TestClear(t *testing.T) {
	b := New(128)
	b.Set(7)
	b.Clear(7)
	if b.Get(7) {
		t.Fatal("clr")
	}
}

func TestCount(t *testing.T) {
	b := New(256)
	b.Set(0)
	b.Set(100)
	b.Set(200)
	if b.Count() != 3 {
		t.Fatal("count", b.Count())
	}
}

func TestOr(t *testing.T) {
	a := New(64)
	b := New(64)
	a.Set(5)
	b.Set(10)
	a.Or(b)
	if !a.Get(5) || !a.Get(10) {
		t.Fatal("or")
	}
}

func TestAnd(t *testing.T) {
	a := New(64)
	b := New(64)
	a.Set(5)
	a.Set(10)
	b.Set(5)
	a.And(b)
	if !a.Get(5) || a.Get(10) {
		t.Fatal("and")
	}
}

func TestLen(t *testing.T) {
	b := New(64)
	if b.Len() != 64 {
		t.Fatal("len")
	}
}

func TestGrow(t *testing.T) {
	b := New(8)
	b.Set(100)
	if !b.Get(100) {
		t.Fatal("grow")
	}
}
