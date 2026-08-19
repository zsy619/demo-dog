package watchx

import (
	"testing"
	"time"
)

func TestSubscribe(t *testing.T) {
	w := New[int]()
	ch, cancel := w.Subscribe(4)
	if w.Subscribers() != 1 {
		t.Fatal("sub")
	}
	w.Publish(1)
	select {
	case v := <-ch:
		if v != 1 {
			t.Fatal("val", v)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	cancel()
	if w.Subscribers() != 0 {
		t.Fatal("unsub")
	}
}

func TestDropOnFull(t *testing.T) {
	w := New[int]()
	ch, cancel := w.Subscribe(1)
	defer cancel()
	w.Publish(1)
	w.Publish(2) // 满，应丢弃
	w.Publish(3) // 满，应丢弃
	v := <-ch
	if v != 1 {
		t.Fatal("drop", v)
	}
}

func TestMultiSubs(t *testing.T) {
	w := New[int]()
	a, ca := w.Subscribe(2)
	defer ca()
	b, cb := w.Subscribe(2)
	defer cb()
	w.Publish(10)
	if (<-a) != 10 || (<-b) != 10 {
		t.Fatal("multi")
	}
}
