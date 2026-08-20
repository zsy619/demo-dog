package dualwal

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestAppendRead(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf, nil)
	if err := w.Append([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := w.Append([]byte("bb")); err != nil {
		t.Fatal(err)
	}
	r := NewReader(&buf)
	a, _ := r.Next()
	if string(a) != "a" {
		t.Fatal("a")
	}
	b, _ := r.Next()
	if string(b) != "bb" {
		t.Fatal("bb")
	}
	if _, err := r.Next(); !errors.Is(err, io.EOF) {
		t.Fatal("EOF")
	}
}

func TestDual(t *testing.T) {
	var primary, mirror bytes.Buffer
	w := New(&primary, &mirror)
	for _, p := range [][]byte{[]byte("x"), []byte("y"), []byte("z")} {
		w.Append(p)
	}
	if primary.Len() != mirror.Len() {
		t.Fatal("mirror match")
	}
	rp := NewReader(&primary)
	rm := NewReader(&mirror)
	for {
		a, err := rp.Next()
		b, err2 := rm.Next()
		if err != err2 {
			t.Fatal("err diff")
		}
		if err != nil {
			break
		}
		if !bytes.Equal(a, b) {
			t.Fatal("diff")
		}
	}
}

func TestCount(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf, nil)
	for i := 0; i < 5; i++ {
		w.Append([]byte("x"))
	}
	if w.Count() != 5 {
		t.Fatal("count")
	}
}

func TestVerify(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf, nil)
	w.Append([]byte("a"))
	w.Append([]byte("b"))
	r := NewReader(&buf)
	n, err := r.Verify()
	if err != nil || n != 2 {
		t.Fatal(err)
	}
}

func TestBadRecord(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf, nil)
	w.Append([]byte("a"))
	// Corrupt the payload bytes.
	data := buf.Bytes()
	data[len(data)-1] ^= 0xFF
	r := NewReader(bytes.NewReader(data))
	if _, err := r.Next(); !errors.Is(err, ErrBadRecord) {
		t.Fatal(err)
	}
}

func TestEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf, nil)
	w.Append([]byte(""))
	r := NewReader(&buf)
	if _, err := r.Next(); !errors.Is(err, ErrBadRecord) && err != nil && err != io.EOF {
		t.Fatal(err)
	}
}
