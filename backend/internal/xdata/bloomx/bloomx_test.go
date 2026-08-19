package bloomx

import "testing"

func TestAddContains(t *testing.T) {
	f := New(100, 0.01)
	f.Add([]byte("hello"))
	if !f.Contains([]byte("hello")) {
		t.Fatal("contains")
	}
}

func TestContainsMissing(t *testing.T) {
	f := New(100, 0.01)
	if f.Contains([]byte("xyz")) {
		t.Fatal("missing")
	}
}

func TestSerialize(t *testing.T) {
	f := New(100, 0.01)
	f.Add([]byte("a"))
	f.Add([]byte("b"))
	b := f.Bytes()
	f2, err := FromBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	if f2.Count() != 2 {
		t.Fatal("count")
	}
	if !f2.Contains([]byte("a")) || !f2.Contains([]byte("b")) {
		t.Fatal("contains")
	}
}

func TestFromBytes_Bad(t *testing.T) {
	if _, err := FromBytes([]byte("x")); err == nil {
		t.Fatal("short")
	}
}

func TestFromBytes_Align(t *testing.T) {
	// 16 字节 hdr + 5 字节 data + 1 字节 h = 22 字节，data=5 不整除
	b := make([]byte, 16)
	b = append(b, 1, 2, 3, 4, 5)
	b = append(b, 1)
	if _, err := FromBytes(b); err == nil {
		t.Fatal("align")
	}
}

func TestSize(t *testing.T) {
	f := New(100, 0.01)
	if f.Size() == 0 {
		t.Fatal("size")
	}
}

func TestHashCount(t *testing.T) {
	f := New(100, 0.01)
	if f.HashCount() < 1 {
		t.Fatal("h")
	}
}
