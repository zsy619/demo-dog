package bitmap

import "testing"

func TestSetGet(t *testing.T) {
	b := New(128)
	b.Set(5)
	if !b.Get(5) {
		t.Fatal("set")
	}
}

func TestClear(t *testing.T) {
	b := New(128)
	b.Set(5)
	b.Clear(5)
	if b.Get(5) {
		t.Fatal("clr")
	}
}

func TestCount(t *testing.T) {
	b := New(128)
	b.Set(1)
	b.Set(2)
	b.Set(3)
	if b.Count() != 3 {
		t.Fatal("count", b.Count())
	}
}

func TestGrow(t *testing.T) {
	b := New(8)
	b.Set(100)
	if !b.Get(100) {
		t.Fatal("grow")
	}
}

func TestLen(t *testing.T) {
	b := New(64)
	if b.Len() != 64 {
		t.Fatal("len")
	}
}
