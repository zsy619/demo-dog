package persistentbuf

import "testing"

func TestAppend(t *testing.T) {
	b := New()
	b.Append([]byte("hello"))
	if b.Len() != 5 {
		t.Fatal("len")
	}
}

func TestBytes(t *testing.T) {
	b := New()
	b.Append([]byte("hi"))
	if string(b.Bytes()) != "hi" {
		t.Fatal("bytes")
	}
}

func TestRead(t *testing.T) {
	b := New()
	b.Append([]byte("abcdef"))
	out := b.Read(3)
	if string(out) != "abc" {
		t.Fatal("read", out)
	}
}

func TestReset(t *testing.T) {
	b := New()
	b.Append([]byte("a"))
	b.Reset()
	if b.Len() != 0 {
		t.Fatal("reset")
	}
}

func TestByte(t *testing.T) {
	b := New()
	b.AppendByte('x')
	if string(b.Bytes()) != "x" {
		t.Fatal("byte")
	}
}
