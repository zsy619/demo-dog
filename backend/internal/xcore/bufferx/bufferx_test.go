package bufferx

import "testing"

func TestWriteRead(t *testing.T) {
	b := New(16)
	b.Write([]byte("hello"))
	out := b.Read(3)
	if string(out) != "hel" {
		t.Fatal("read", out)
	}
	if b.Len() != 2 {
		t.Fatal("len", b.Len())
	}
}

func TestBytes(t *testing.T) {
	b := New(8)
	b.Write([]byte("abc"))
	if string(b.Bytes()) != "abc" {
		t.Fatal("bytes")
	}
}

func TestReset(t *testing.T) {
	b := New(8)
	b.Write([]byte("x"))
	b.Reset()
	if b.Len() != 0 {
		t.Fatal("reset")
	}
}

func TestWriteByte(t *testing.T) {
	b := New(8)
	b.WriteByte('a')
	if b.Len() != 1 {
		t.Fatal("byte")
	}
}
