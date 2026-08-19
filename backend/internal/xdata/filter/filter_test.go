package filter

import (
	"testing"
)

func TestAddContains(t *testing.T) {
	f := New(64)
	f.Add([]byte("hello"))
	if !f.Contains([]byte("hello")) {
		t.Fatal("应存在")
	}
}

func TestContainsMissing(t *testing.T) {
	f := New(1024)
	if f.Contains([]byte("x")) {
		t.Fatal("应不存在")
	}
}

func TestReset(t *testing.T) {
	f := New(64)
	f.Add([]byte("k"))
	f.Reset()
	if f.Contains([]byte("k")) {
		t.Fatal("应清空")
	}
}

func TestLen(t *testing.T) {
	f := New(64)
	if f.Len() != 64 {
		t.Fatal("len")
	}
}

func TestCounterFilter_Estimate(t *testing.T) {
	c := &CounterFilter{Filter: *New(64)}
	c.Add([]byte("k"))
	c.Add([]byte("k"))
	if c.Estimate([]byte("k")) < 1 {
		t.Fatal("estimate")
	}
}
