package circular

import "testing"

func TestPushPop(t *testing.T) {
	b := New(4)
	b.Push(1)
	b.Push(2)
	if v, ok := b.Pop(); !ok || v.(int) != 1 {
		t.Fatal("pop1")
	}
	if v, _ := b.Pop(); v.(int) != 2 {
		t.Fatal("pop2")
	}
}

func TestPop_Empty(t *testing.T) {
	b := New(4)
	if _, ok := b.Pop(); ok {
		t.Fatal("empty")
	}
}

func TestOverflow(t *testing.T) {
	b := New(2)
	b.Push(1)
	b.Push(2)
	b.Push(3)
	if b.Len() != 2 {
		t.Fatal("len", b.Len())
	}
	v, _ := b.Pop()
	if v.(int) != 2 {
		t.Fatal("fifo", v)
	}
}

func TestReset(t *testing.T) {
	b := New(4)
	b.Push(1)
	b.Reset()
	if b.Len() != 0 {
		t.Fatal("reset")
	}
}

func TestSnapshot(t *testing.T) {
	b := New(3)
	b.Push(1)
	b.Push(2)
	b.Push(3)
	s := b.Snapshot()
	if len(s) != 3 || s[0].(int) != 1 {
		t.Fatal("snap", s)
	}
}

func TestCap(t *testing.T) {
	b := New(5)
	if b.Cap() != 5 {
		t.Fatal("cap")
	}
}
