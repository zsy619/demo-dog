package optional

import "testing"

func TestSome(t *testing.T) {
	o := Some(42)
	if !o.IsPresent() {
		t.Fatal("present")
	}
	if o.Value() != 42 {
		t.Fatal("v")
	}
}

func TestNone(t *testing.T) {
	o := None[int]()
	if o.IsPresent() {
		t.Fatal("should be none")
	}
}

func TestOrElse(t *testing.T) {
	o := None[int]()
	if o.OrElse(99) != 99 {
		t.Fatal("else")
	}
	o = Some(1)
	if o.OrElse(99) != 1 {
		t.Fatal("or")
	}
}

func TestFromPtr(t *testing.T) {
	x := 10
	o := FromPtr(&x)
	if !o.IsPresent() || o.Value() != 10 {
		t.Fatal("ptr")
	}
	o = FromPtr[int](nil)
	if o.IsPresent() {
		t.Fatal("nil ptr")
	}
}

func TestMap(t *testing.T) {
	o := Map(Some(2), func(v int) int { return v * 10 })
	if o.Value() != 20 {
		t.Fatal("map")
	}
	n := Map(None[int](), func(v int) int { return v })
	if n.IsPresent() {
		t.Fatal("map none")
	}
}

func TestPtr(t *testing.T) {
	o := Some(5)
	p := o.Ptr()
	if p == nil || *p != 5 {
		t.Fatal("ptr v")
	}
	if None[int]().Ptr() != nil {
		t.Fatal("ptr none")
	}
}
