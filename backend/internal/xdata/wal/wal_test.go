package wal

import (
	"path/filepath"
	"testing"
)

func openTmp(t *testing.T) *WAL {
	t.Helper()
	w, err := Open(filepath.Join(t.TempDir(), "wal.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

func TestAppendRead(t *testing.T) {
	w := openTmp(t)
	for i := uint64(1); i <= 3; i++ {
		if err := w.Append(i, []byte("hi")); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	r, err := NewReader(w.path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for i := uint64(1); i <= 3; i++ {
		seq, p, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		if seq != i || string(p) != "hi" {
			t.Fatalf("seq %d payload %s", seq, p)
		}
	}
}

func TestReader_EOF(t *testing.T) {
	w := openTmp(t)
	r, err := NewReader(w.path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, _, err := r.Next(); err == nil {
		t.Fatal("expected EOF")
	}
}

func TestSnapshot(t *testing.T) {
	w := openTmp(t)
	w.Append(1, []byte("a"))
	w.Append(2, []byte("b"))
	w.Append(3, []byte("c"))
	if err := w.WriteSnapshot(2, []byte("snap")); err != nil {
		t.Fatal(err)
	}
	s := w.LastSnapshot()
	if s == nil || s.Seq != 2 || string(s.Payload) != "snap" {
		t.Fatal("snapshot mismatch")
	}
	w.Append(4, []byte("d"))
	w.Close()
	r, _ := NewReader(w.path)
	defer r.Close()
	_, p1, _ := r.Next()
	// First entry is the snapshot frame.
	if len(p1) == 0 || string(p1) == "d" {
		t.Fatalf("unexpected first: %s", p1)
	}
}

func TestEncodeDecodeFrame(t *testing.T) {
	f := encodeFrame(42, []byte("hello"))
	seq, p, err := decodeFrame(f)
	if err != nil {
		t.Fatal(err)
	}
	if seq != 42 || string(p) != "hello" {
		t.Fatal("decode mismatch")
	}
}

func TestDecodeFrame_BadMagic(t *testing.T) {
	f := []byte("XX\x00\x00\x00\x00\x00")
	if _, _, err := decodeFrame(f); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeFrame_ShortBuffer(t *testing.T) {
	if _, _, err := decodeFrame([]byte("ab")); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpen_MissingDir(t *testing.T) {
	if _, err := Open("/no/such/dir/wal"); err == nil {
		t.Fatal("expected error")
	}
}
