package windowx

import "testing"

func TestAdd(t *testing.T) {
	w := New(5)
	w.Add(3)
	if w.Sum() != 3 {
		t.Fatal("sum", w.Sum())
	}
}

func TestTick(t *testing.T) {
	w := New(3)
	w.Add(5)
	w.Tick()
	w.Add(2)
	w.Tick()
	w.Add(3)
	if w.Sum() != 10 {
		t.Fatal("tick sum", w.Sum())
	}
}

func TestReset(t *testing.T) {
	w := New(5)
	w.Add(5)
	w.Reset()
	if w.Sum() != 0 {
		t.Fatal("reset")
	}
}

func TestLen(t *testing.T) {
	w := New(7)
	if w.Len() != 7 {
		t.Fatal("len")
	}
}
