package rwmap

import (
	"sync"
	"testing"
)

func TestPutGet(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	v, ok := m.Get("a")
	if !ok || v != 1 {
		t.Fatal("get")
	}
}

func TestDelete(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Delete("a")
	if _, ok := m.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestLen(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	if m.Len() != 1 {
		t.Fatal("len")
	}
}

func TestRange(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	sum := 0
	m.Range(func(_ string, v int) bool { sum += v; return true })
	if sum != 3 {
		t.Fatal("range")
	}
}

func TestSnapshot(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	s := m.Snapshot()
	if s["a"] != 1 {
		t.Fatal("snap")
	}
}

func TestConcurrent(t *testing.T) {
	m := New[int, int]()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			m.Set(i, i)
		}(i)
		go func(i int) {
			defer wg.Done()
			m.Get(i)
		}(i)
	}
	wg.Wait()
}
