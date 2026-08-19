package cdc

import (
	"testing"
	"time"
)

func TestRecorder_Put(t *testing.T) {
	r := New(8)
	ev := r.Put("k", []byte("v"))
	if ev.Op != OpPut || ev.Key != "k" {
		t.Fatal("ev")
	}
	if ev.Seq != 1 {
		t.Fatal("seq")
	}
}

func TestRecorder_Delete(t *testing.T) {
	r := New(8)
	ev := r.Delete("k")
	if ev.Op != OpDelete {
		t.Fatal("del")
	}
}

func TestSubscribe(t *testing.T) {
	r := New(8)
	ch, unsub := r.Subscribe(4)
	defer unsub()
	r.Put("x", []byte("1"))
	select {
	case ev := <-ch:
		if ev.Key != "x" {
			t.Fatal("sub")
		}
	case <-time.After(time.Second):
		t.Fatal("超时")
	}
}

func TestHistory(t *testing.T) {
	r := New(8)
	r.Put("a", nil)
	r.Put("b", nil)
	h := r.History()
	if len(h) != 2 {
		t.Fatal("历史")
	}
}

func TestStats(t *testing.T) {
	r := New(8)
	r.Put("a", nil)
	if r.Stats().Seq != 1 {
		t.Fatal("seq")
	}
}

func TestTail(t *testing.T) {
	r := New(8)
	for i := 0; i < 5; i++ {
		r.Put(string(rune('a'+i)), nil)
	}
	tail := r.Tail(2)
	if len(tail) != 2 {
		t.Fatal("tail")
	}
}
