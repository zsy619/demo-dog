package journal

import (
	"errors"
	"testing"
)

func TestPutDelete(t *testing.T) {
	l := New(10)
	if err := l.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := l.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if l.Len() != 2 {
		t.Fatal("len")
	}
}

func TestFull(t *testing.T) {
	l := New(2)
	l.Put("a", nil)
	l.Put("b", nil)
	if err := l.Put("c", nil); !errors.Is(err, ErrFull) {
		t.Fatal("应满")
	}
}

func TestFilter(t *testing.T) {
	l := New(10)
	l.Put("a", nil)
	l.Put("b", nil)
	l.Put("a", nil)
	if len(l.Filter("a")) != 2 {
		t.Fatal("filter")
	}
}

func TestLatest(t *testing.T) {
	l := New(10)
	l.Put("a", []byte("1"))
	l.Put("a", []byte("2"))
	m := l.Latest()
	if string(m["a"].Value) != "2" {
		t.Fatal("latest")
	}
}

func TestClear(t *testing.T) {
	l := New(10)
	l.Put("a", nil)
	l.Clear()
	if l.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestRange(t *testing.T) {
	l := New(10)
	l.Put("a", nil)
	if len(l.Range()) != 1 {
		t.Fatal("range")
	}
}
