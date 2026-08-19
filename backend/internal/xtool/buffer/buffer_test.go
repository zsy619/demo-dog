package buffer

import (
	"bytes"
	"errors"
	"testing"
)

func TestWriteRead(t *testing.T) {
	r := New(8)
	r.Write([]byte("hi"))
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || string(buf[:2]) != "hi" {
		t.Fatal("read")
	}
}

func TestRead_Empty(t *testing.T) {
	r := New(4)
	buf := make([]byte, 4)
	if _, err := r.Read(buf); !errors.Is(err, ErrEmpty) {
		t.Fatal("应 ErrEmpty")
	}
}

func TestOverwrite(t *testing.T) {
	r := New(4)
	r.Write([]byte("abcdef")) // 容量 4，应保留 cdef
	if r.Len() != 4 {
		t.Fatal("len")
	}
	if !bytes.Equal(r.Bytes(), []byte("cdef")) {
		t.Fatal("overwrite")
	}
}

func TestLenCap(t *testing.T) {
	r := New(8)
	if r.Cap() != 8 {
		t.Fatal("cap")
	}
	r.Write([]byte("x"))
	if r.Len() != 1 {
		t.Fatal("len")
	}
}

func TestClear(t *testing.T) {
	r := New(4)
	r.Write([]byte("ab"))
	r.Clear()
	if r.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestBytes(t *testing.T) {
	r := New(8)
	r.Write([]byte("hello"))
	if !bytes.Equal(r.Bytes(), []byte("hello")) {
		t.Fatal("bytes")
	}
}
