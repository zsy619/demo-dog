package workqueue

import (
	"testing"
	"time"
)

func TestAddGet(t *testing.T) {
	q := New()
	q.Add("a", 1, time.Now())
	it, ok := q.Get()
	if !ok || it.Key != "a" {
		t.Fatal("get")
	}
}

func TestDelayed(t *testing.T) {
	q := New()
	q.Add("a", 1, time.Now().Add(50*time.Millisecond))
	start := time.Now()
	it, _ := q.Get()
	if time.Since(start) < 40*time.Millisecond {
		t.Fatal("未延迟")
	}
	_ = it
}

func TestOverride(t *testing.T) {
	q := New()
	q.Add("a", 1, time.Now().Add(time.Hour))
	q.Add("a", 2, time.Now())
	it, _ := q.Get()
	if it.Value.(int) != 2 {
		t.Fatal("override")
	}
}

func TestLen(t *testing.T) {
	q := New()
	q.Add("a", 1, time.Now())
	if q.Len() != 1 {
		t.Fatal("len")
	}
}

func TestAddAfter(t *testing.T) {
	q := New()
	q.AddAfter("a", 1, 20*time.Millisecond)
	it, _ := q.Get()
	if it.Key != "a" {
		t.Fatal("after")
	}
}

func TestDone(t *testing.T) {
	q := New()
	q.Add("a", 1, time.Now())
	q.Done("a")
	if _, ok := q.key["a"]; ok {
		t.Fatal("done")
	}
}
