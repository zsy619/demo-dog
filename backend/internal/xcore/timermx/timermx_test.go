package timermx

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAfter(t *testing.T) {
	m := New()
	var n atomic.Int32
	m.After(20*time.Millisecond, func() { n.Add(1) })
	time.Sleep(60 * time.Millisecond)
	if n.Load() != 1 {
		t.Fatal("after")
	}
	m.Shutdown()
}

func TestEvery(t *testing.T) {
	m := New()
	defer m.Shutdown()
	var n atomic.Int32
	m.Every(20*time.Millisecond, func() { n.Add(1) })
	time.Sleep(70 * time.Millisecond)
	if n.Load() < 2 {
	t.Fatal("every")
	}
}

func TestCancel(t *testing.T) {
	m := New()
	defer m.Shutdown()
	var n atomic.Int32
	id := m.After(50*time.Millisecond, func() { n.Add(1) })
	m.Cancel(id)
	time.Sleep(80 * time.Millisecond)
	if n.Load() != 0 {
		t.Fatal("cancel")
	}
}

func TestLen(t *testing.T) {
	m := New()
	if m.Len() != 0 {
		t.Fatal("empty")
	}
	m.After(time.Hour, func() {})
	if m.Len() != 1 {
		t.Fatal("len")
	}
	m.Shutdown()
}
