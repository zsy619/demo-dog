package tarx

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestBuildParseRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	h := &Header{Name: "hello.txt", Size: 5, TypeFlag: 78, Mode: 420, UName: "alice"}
	if err := w.Write(h, []byte("world")); err != nil {
		t.Fatal(err)
	}
	w.Close()
	r := NewReader(&buf)
	e, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if e.Header.Name != "hello.txt" {
		t.Fatal("name")
	}
	body, _ := io.ReadAll(e.Body)
	if string(body) != "world" {
		t.Fatal("body")
	}
}

func TestParseHeader_BadSize(t *testing.T) {
	if _, err := ParseHeader([]byte{}); err != ErrTruncated {
		t.Fatal(err)
	}
}

func TestParseHeader_BadChecksum(t *testing.T) {
	h := &Header{Name: "x", TypeFlag: 78, Size: 0, Mode: 420}
	hdr := BuildHeader(h)
	for i := 148; i < 156; i++ {
		hdr[i] = 0
	}
	if _, err := ParseHeader(hdr); !errors.Is(err, ErrBadChecksum) {
		t.Fatal(err)
	}
}

func TestNext_EOF(t *testing.T) {
	var buf bytes.Buffer
	r := NewReader(&buf)
	if _, err := r.Next(); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

func TestNext_ZeroBlock(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(make([]byte, 512))
	r := NewReader(&buf)
	if _, err := r.Next(); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

func TestWrite_MultiEntry(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for _, name := range []string{"a", "b", "c"} {
		if err := w.Write(&Header{Name: name, TypeFlag: 78}, []byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	r := NewReader(&buf)
	for _, name := range []string{"a", "b", "c"} {
		e, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		if e.Header.Name != name {
			t.Fatal("name")
		}
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatal("expected EOF")
	}
}

func TestPadOctal(t *testing.T) {
	b := padOctal(8, 11)
	if len(b) != 12 {
		t.Fatal("len")
	}
}

func TestCString(t *testing.T) {
	if cString([]byte{'a', 'b', 0, 'c'}) != "ab" {
		t.Fatal("cstring")
	}
	if cString([]byte{'a', 'b'}) != "ab" {
		t.Fatal("no null")
	}
}
