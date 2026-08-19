package runtimex

import (
	"testing"
	"time"
)

func TestRecordPanic(t *testing.T) {
	r := New(8)
	r.RecordPanic("boom", "test")
	if r.Stats().Panics != 1 {
		t.Fatal("panic")
	}
}

func TestRecordWarn(t *testing.T) {
	r := New(8)
	r.RecordWarn("warn", "test")
	if r.Stats().Warns != 1 {
		t.Fatal("warn")
	}
}

func TestHistory(t *testing.T) {
	r := New(8)
	r.RecordWarn("a", "x")
	r.RecordPanic("b", "y")
	if len(r.History()) != 2 {
		t.Fatal("history")
	}
}

func TestCapacity(t *testing.T) {
	r := New(2)
	for i := 0; i < 5; i++ {
		r.RecordWarn("x", "y")
	}
	h := r.History()
	if len(h) != 2 {
		t.Fatal("cap")
	}
}

func TestGoSafe(t *testing.T) {
	r := New(8)
	r.GoSafe(func() { panic("oops") })
	time.Sleep(50 * time.Millisecond)
	if r.Stats().Panics != 1 {
		t.Fatal("gossafe")
	}
}

func TestGoSafe_NoPanic(t *testing.T) {
	r := New(8)
	r.GoSafe(func() {})
	time.Sleep(20 * time.Millisecond)
	if r.Stats().Panics != 0 {
		t.Fatal("不应有 panic")
	}
}

func TestClear(t *testing.T) {
	r := New(8)
	r.RecordWarn("x", "y")
	r.Clear()
	if len(r.History()) != 0 {
		t.Fatal("clear")
	}
}
