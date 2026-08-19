package boxx

import "testing"

func TestNew(t *testing.T) {
	b := New(42)
	if b.Value() != 42 {
		t.Fatal("v")
	}
}

func TestSet(t *testing.T) {
	b := New(0)
	b.Set(10)
	if b.Value() != 10 {
		t.Fatal("set")
	}
}

func TestMap(t *testing.T) {
	b := New(2)
	doubled := Map(b, func(v int) int { return v * 2 })
	if doubled.Value() != 4 {
		t.Fatal("map")
	}
}

func TestOrElse(t *testing.T) {
	b := New(7)
	if OrElse(b, true, 99) != 7 {
		t.Fatal("ok")
	}
	if OrElse(b, false, 99) != 99 {
		t.Fatal("else")
	}
}

func TestPointerBox(t *testing.T) {
	type Point struct{ X, Y int }
	b := New(&Point{X: 1, Y: 2})
	if b.Value().X != 1 {
		t.Fatal("ptr")
	}
}
