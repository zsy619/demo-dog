package bloom

import "testing"

func TestAddContains(t *testing.T) {
	f := New(1000, 0.01)
	f.Add([]byte("hello"))
	if !f.Contains([]byte("hello")) {
		t.Fatal("应存在")
	}
}

func TestContainsMissing(t *testing.T) {
	f := New(1000, 0.01)
	if f.Contains([]byte("nope")) {
		t.Fatal("应不存在")
	}
}

func TestReset(t *testing.T) {
	f := New(1000, 0.01)
	f.Add([]byte("k"))
	f.Reset()
	if f.Contains([]byte("k")) {
		t.Fatal("reset")
	}
}

func TestSize(t *testing.T) {
	f := New(100, 0.01)
	if f.Size() == 0 {
		t.Fatal("size")
	}
}

func TestHashCount(t *testing.T) {
	f := New(1000, 0.01)
	if f.HashCount() < 1 {
		t.Fatal("hash")
	}
}
