package framing

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type readOnly struct{ b []byte }

func (r *readOnly) Read(p []byte) (int, error) {
	n := copy(p, r.b)
	r.b = r.b[n:]
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (*readOnly) Write([]byte) (int, error) { return 0, io.EOF }

func TestRoundTrip_Small(t *testing.T) {
	var buf bytes.Buffer
	c := New(&buf, 0)
	if err := c.Write(OpText, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	c2 := New(&buf, 0)
	f, err := c2.Read()
	if err != nil {
		t.Fatal(err)
	}
	if f.Op != OpText || string(f.Payload) != "hi" {
		t.Fatal("frame")
	}
}

func TestRoundTrip_Extended16(t *testing.T) {
	var buf bytes.Buffer
	c := New(&buf, 0)
	payload := bytes.Repeat([]byte("a"), 200)
	if err := c.Write(OpBinary, payload); err != nil {
		t.Fatal(err)
	}
	c2 := New(&buf, 0)
	f, err := c2.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.Payload, payload) {
		t.Fatal("payload")
	}
}

func TestRoundTrip_Extended64(t *testing.T) {
	var buf bytes.Buffer
	c := New(&buf, 0)
	payload := bytes.Repeat([]byte("x"), 70000)
	if err := c.Write(OpBinary, payload); err != nil {
		t.Fatal(err)
	}
	c2 := New(&buf, 0)
	f, err := c2.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.Payload, payload) {
		t.Fatal("payload")
	}
}

func TestEmpty(t *testing.T) {
	var buf bytes.Buffer
	c := New(&buf, 0)
	if err := c.Write(OpPing, nil); err != nil {
		t.Fatal(err)
	}
	c2 := New(&buf, 0)
	f, err := c2.Read()
	if err != nil {
		t.Fatal(err)
	}
	if f.Op != OpPing || len(f.Payload) != 0 {
		t.Fatal("empty")
	}
}

func TestTooLarge(t *testing.T) {
	var buf bytes.Buffer
	c := New(&buf, 10)
	if err := c.Write(OpBinary, bytes.Repeat([]byte("a"), 100)); err != nil {
		t.Fatal(err)
	}
	c2 := New(&buf, 10)
	if _, err := c2.Read(); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatal(err)
	}
}

func TestClose(t *testing.T) {
	var buf bytes.Buffer
	c := New(&buf, 0)
	c.Write(OpClose, []byte("bye"))
	c2 := New(&buf, 0)
	f, _ := c2.Read()
	if f.Op != OpClose {
		t.Fatal("close")
	}
}

func TestTruncated(t *testing.T) {
	var buf bytes.Buffer
	c := New(&buf, 0)
	c.Write(OpText, []byte("hello"))
	c2 := New(&readOnly{b: buf.Bytes()[:4]}, 0)
	if _, err := c2.Read(); err == nil {
		t.Fatal("expected error")
	}
}
